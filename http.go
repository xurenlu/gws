package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/yuanfenxi/ledis"
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

// serveWs handles websocket requests from the peer.
func ServeStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Println("new Client connected \t uri:", r.RequestURI, ",remote Addr:", r.RemoteAddr)
	message := "status ok!"
	w.Write([]byte(message))
}

func ServeHistoryMessage(w http.ResponseWriter, r *http.Request) {
	fmt.Println("new Client connected \t uri:", r.RequestURI, ",remote Addr:", r.RemoteAddr)
	setCORS(w)
	if "POST" != r.Method {
		w.Write([]byte("{'code':403,'msg':'only post  supported'}"))
		return
	}
	r.ParseForm()
	groupName := ""
	fromPosition := ""
	size := "1000"
	if r.Form["groupName"] != nil {
		groupName = strings.Join(r.Form["groupName"], "")
	}
	if r.Form["position"] != nil {
		fromPosition = strings.Join(r.Form["position"], "")
	}
	if r.Form["size"] != nil {
		size = strings.Join(r.Form["size"], "")
	}
	if groupName == "" {
		w.Write([]byte("{'code':403,'msg':'groupName missing'}"))
		return
	}
	//log.Println("got history query")
	if groupName != "" {
		var theClient *redis.Client
		theClient = redis.NewClient(&redis.Options{
			Addr:     LedisAddr,
			PoolSize: 16,
		})
		defer theClient.Close()
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
				w.WriteHeader(500)
				hisResult := HistoryResult{}
				hisResult.Code = 500
				hisResult.Message = "can't stat length of group" + lengthError.Error()
				jsonObject, _ := json.Marshal(hisResult)
				w.Write(jsonObject)
				return
			}
			returnObj, _ = theClient.LRange(groupName, int64(currentLength)+int64(positionInt), int64(currentLength)+int64(positionInt+sizeInt)-1).Result()
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
		jsonData, er := json.Marshal(historyResult)
		if er != nil {
			w.Write([]byte("{'code':403,'msg':'json marshal failed;'}"))
		} else {
			w.Write(jsonData)
		}
	} else {

	}
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
	//if er!= nil {
	//	w.Write([]byte( er.Error()))
	//	w.Write([]byte("{'code':500}"))
	//}else{
	//	w.Write(data)
	//}

}

func ServeCancel(w http.ResponseWriter, r *http.Request) {
	fmt.Println("new Client connected \t uri:", r.RequestURI, ",remote Addr:", r.RemoteAddr, ",Method:", r.Method)
	setCORS(w)

	//\n",r.URL.Path,r.Method)
	uuid := ""

	var theClient *redis.Client
	theClient = redis.NewClient(&redis.Options{
		Addr:     LedisAddr,
		PoolSize: 16,
	})
	defer theClient.Close()

	if "POST" == r.Method {
		r.ParseForm()
		groupName := ""

		if r.Form["groupName"] != nil {
			groupName = strings.Join(r.Form["groupName"], "")
		}
		if groupName == "" {
			w.Write([]byte("{'code':403,'msg':'groupName missing'}"))
			return
		}
		if r.Form["uuid"] != nil {
			uuid = strings.Join(r.Form["uuid"], "")
		}

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
					w.Write([]byte("{'code':200,'msg':'uuid found and saved in blacklist:" + hex.EncodeToString(sum) + "'}"))
					return
				}
			}
		}
		w.Write([]byte("{'code':200,'msg':'uuid not found in latest 50 items;'}"))

	}
	if "GET" == r.Method {
		w.Write([]byte("only POST method supported"))
	}
}

func serveTTL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Write([]byte("{'code':403,'msg':'only post  supported'}"))
		return
	}

	r.ParseForm()
	grp := r.URL.Path
	ttl := -1
	if r.Form["groupName"] == nil {
		w.Write([]byte("{'code':403,'msg':'groupname missing'}"))
		return
	}
	grp = strings.Join(r.Form["groupName"], "")
	if r.Form["ttl"] != nil {
		var er error
		ttl, er = strconv.Atoi(strings.Join(r.Form["ttl"], ""))
		if er != nil {
			log.Println("got an error while parse ttl;")
			w.Write([]byte("{'code':403,'msg':'ttl should be great than 0 '}"))
			return
		}
	}
	if ttl == -1 {
		w.Write([]byte("{'code':403,'msg':'ttl should be great than 0 '}"))
	}
	client := redis.NewClient(&redis.Options{
		Addr:     LedisAddr,
		PoolSize: 128,
	})

	defer client.Close()
	result, errorOfLedis := client.LExpire(grp, ttl).Result()
	log.Printf("set ttl result:%d", result)
	if errorOfLedis == nil {
		w.Write([]byte("{'code':200,'msg':'suceed'}"))
	} else {
		w.Write([]byte("{'code':405,'msg':'set ttl failed'}"))
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	fmt.Println("new Client connected \t uri:", r.RequestURI, ",remote Addr:", r.RemoteAddr, ",Method:", r.Method)
	//\n",r.URL.Path,r.Method)
	if "POST" == r.Method {

		grp := ""

		grp = r.URL.Path
		var data []byte
		n, er := r.Body.Read(data)
		if er != nil {
			w.Write([]byte("{'code':500,'msg':'data missing'}"))
			return
		}

		if n > 0 {
			handleNewWsData(hub, nil, data, grp)
			//message := MessageToSend{groupName: grp, broadcast: []byte(data)}
			//hub.ChanToBroadCast <- message
			//hub.ChanToSaveToLedis <- message
			w.Write([]byte("{'code':200,'group':" + grp + "}"))
		} else {
			w.Write([]byte("{'code':403,'msg':'data missing'}"))
		}
		return
	} else {
		//.Println("new client .")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			//log.Println(err)
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
}
