package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	redis "github.com/yuanfenxi/ledis"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func foundInList(item string, blackList []string) bool {
	h5 := md5.New()
	h5.Write([]byte(item))
	sum := hex.EncodeToString(h5.Sum(nil))
	for i := 0; i < len(blackList); i++ {
		if sum == blackList[i] {
			return true
		}
	}
	return false
}

func setCORS(w http.ResponseWriter) {
	w.Header().Set("X-Ws-Version", "1.0.0")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "OPTION,OPTIONS,GET,POST,PATCH,DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "authorization,rid,Authorization,Content-Type,Accept,x-requested-with，Origin, X-Requested-With, Content-Type,User-Agent,Referer")
	w.Header().Set("Access-Control-Expose-Headers", "Authorization")
}

// ServeStatus
// serveWs handles websocket requests from the peer.
func ServeStatus(c *gin.Context) {
	log.Println("new Client connected \t uri:", c.Request.RequestURI, ",remote Addr:", c.Request.RemoteAddr)
	message := "status ok!"
	c.String(http.StatusOK, message)
}

func ServeHistoryMessage(c *gin.Context) {
	r := c.Request
	fmt.Println("new Client connected \t uri:", r.RequestURI, ",remote Addr:", r.RemoteAddr)
	groupName := c.Param("groupName")
	fromPosition := c.Param("position")
	size := c.Param("size")
	if groupName == "" {
		c.JSON(http.StatusForbidden, gin.H{"msg": "groupName missing"})
		return
	}
	var theClient *redis.Client
	theClient = redis.NewClient(&redis.Options{
		Addr:     LedisAddr,
		PoolSize: 16,
	})
	defer func(theClient *redis.Client) {
		err := theClient.Close()
		if err != nil {
			fmt.Println("redis close error:", err.Error())
		}
	}(theClient)
	var positionInt int
	if fromPosition == "" {
		fromPosition, _ = theClient.Get("pos." + groupName).Result()
	}
	positionInt, convertErr := strconv.Atoi(fromPosition)
	if convertErr != nil {
		positionInt = 0
	}

	sizeInt, convertSizeErr := strconv.Atoi(size)
	if convertSizeErr != nil {
		sizeInt = 1000
	}
	var returnObj []string
	if positionInt < 0 {
		currentLength, lengthError := theClient.LLen(groupName).Result()
		if lengthError != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "can't stat length of group" + lengthError.Error()})
			return
		}
		returnObj, _ = theClient.LRange(groupName, currentLength+int64(positionInt), currentLength+int64(positionInt+sizeInt)-1).Result()
	} else {
		returnObj, _ = theClient.LRange(groupName, int64(positionInt), int64(positionInt+sizeInt)-1).Result()
	}

	historyResult := HistoryResult{}
	historyResult.Code = 200
	historyResult.Position = positionInt

	blackResult, _ := theClient.SMembers(blackPrefix + groupName).Result()

	filteredResult := returnObj[:0]
	/**
	读取Redis里的黑名单;
	*/
	for i := 0; i < len(returnObj); i++ {
		if foundInList(returnObj[i], blackResult) {

		} else {
			filteredResult = append(filteredResult, returnObj[i])
		}
	}
	historyResult.Data = filteredResult
	c.JSON(200, historyResult)
	return
}

func ServeGroups(hub *Hub, w http.ResponseWriter, r *http.Request) {
	//data,er := json.Marshal(hub.groupClients)
	fmt.Println("new Client connected \t uri:", r.RequestURI, ",remote Addr:", r.RemoteAddr)
	r.ParseForm()
	if r.Form["secret"] == nil {
		w.Write([]byte("secret missing"))
		return
	}
	postSecret := strings.Join(r.Form["secret"], "")

	if postSecret != secret {
		w.Write([]byte("secret not match"))
		return
	}

	for group, _ := range hub.groupClients {
		w.Write([]byte(group + "\n"))

	}

}

func ServeCancel(c *gin.Context) {
	r := c.Request
	fmt.Println("new Client connected \t uri:", r.RequestURI, ",remote Addr:", r.RemoteAddr, ",Method:", r.Method)

	//\n",r.URL.Path,r.Method)
	uuid := ""

	var theClient *redis.Client
	theClient = redis.NewClient(&redis.Options{
		Addr:     LedisAddr,
		PoolSize: 16,
	})
	defer func(theClient *redis.Client) {
		err := theClient.Close()
		if err != nil {
			fmt.Println("redis close error:", err.Error())
		}
	}(theClient)

	groupName := c.Param("groupName")

	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "groupName missing"})
		return
	}
	uuid = c.Param("uuid")

	positionInt := 0
	sizeInt := 200
	returnObj, _ := theClient.LRange(groupName, int64(positionInt), int64(positionInt+sizeInt)-1).Result()
	for i := 0; i < len(returnObj); i++ {
		var messageObj Message
		marshalErr := json.Unmarshal([]byte(returnObj[i]), &messageObj)
		if marshalErr == nil {
			if messageObj.Uuid == uuid {
				md5Hash := md5.New()
				md5Hash.Write([]byte(returnObj[i]))
				fmt.Println("uuid found,save in LList:" + blackPrefix + groupName)
				sum := md5Hash.Sum(nil)
				//把这个维护到黑名单;
				theClient.SAdd(blackPrefix+groupName, hex.EncodeToString(sum))
				c.JSON(http.StatusOK, gin.H{"msg": "uuid found and saved in blacklist:" + hex.EncodeToString(sum)})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"msg": "uuid not found in latest 50 items;"})
}

func serveTTL(c *gin.Context) {
	ttl := -1
	grp := c.Param("groupName")
	if grp == "" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "groupName missing"})
		return
	}

	ttlInParam := c.Param("ttl")
	if ttlInParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "ttl missing"})
		return
	}

	var er error
	ttl, er = strconv.Atoi(ttlInParam)
	if er != nil {
		log.Println("got an error while parse ttl;")
		c.JSON(http.StatusBadRequest, gin.H{"msg": "ttl should be a number"})
		return
	}

	if ttl < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "ttl should be a number and great than 0"})
		return
	}
	client := redis.NewClient(&redis.Options{
		Addr:     LedisAddr,
		PoolSize: 128,
	})

	defer client.Close()
	result, errorOfLedis := client.LExpire(grp, ttl).Result()
	log.Printf("set ttl result:%d", result)
	if errorOfLedis == nil {
		c.JSON(http.StatusOK, gin.H{"msg": "set ttl succeed"})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "set ttl failed:" + errorOfLedis.Error()})
	}
}
func servePost(hub *Hub, c *gin.Context) {
	r := c.Request

	grp := ""

	grp = r.URL.Path

	data, er := ioutil.ReadAll(r.Body)
	if er != nil {
		c.JSON(500, gin.H{
			"code": 500,
			"msg":  er})
	}

	if len(data) > 0 {
		handleNewWsData(hub, nil, data, grp)
		//message := MessageToSend{groupName: grp, broadcast: []byte(data)}
		//hub.ChanToBroadCast <- message
		//hub.ChanToSaveToLedis <- message
		c.JSON(200, gin.H{
			"code":  200,
			"group": grp})
	} else {
		c.JSON(406, gin.H{
			"code": 406, "msg": "data missing"})
	}
}

// serveWs handles websocket requests from the peer.
func serveGet(hub *Hub, c *gin.Context) {
	r := c.Request
	w := c.Writer
	fmt.Println("new Client connected \t uri:", r.RequestURI, ",remote Addr:", r.RemoteAddr, ",Method:", r.Method)
	//\n",r.URL.Path,r.Method)
	//.Println("new client .")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	//w.Write([]byte("Hello websocket"))
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 8), groupName: r.URL.Path}
	client.hub.register <- client
	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.WritePump()
	go client.ReadPump()
}
