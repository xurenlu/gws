package main

import (
	"crypto/md5"
	"encoding/hex"

	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
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
