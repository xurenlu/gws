package main

import (
	"flag"
	"fmt"
	"github.com/yuanfenxi/ledis"
	"log"
	"net/http"
	"sync"
	"time"
)

var lock4Connections sync.RWMutex
var LedisAddr string
var addr = flag.String("addr", ":4998", "http service address")
var redisClient *redis.Client
var secret string

const (
	blackPrefix = "$$/black/$$"
)

type GroupClients map[*Client]bool

type MessageToSend struct {
	groupName string
	broadcast []byte
	client    *Client
}
type QueryArgument struct {
	Uuid string `json:"uuid"`
}
type MessageBody struct {
	Data string `json:"data"`
	Sent bool   `json:"sent"`
	Hash string `json:"hash"`
}
type MessageType int

const (
	MessageTypeCommand MessageType = 1
	MessageTypeData    MessageType = 2

	MessageCommandServerAck MessageCommand = 1
	/**
	  const MessageCommandQuery MessageCommand = 2
	  const MessageCommandCustomerAck MessageCommand = 3
	  const MessageCommandWatiressAck MessageCommand = 4
	*/
	MessageCommandPingPong MessageCommand = 5
)

type MessageCommand int
type PullArgument struct {
	Offset int `json:"offset"`
	Size   int `json:"size"`
}
type Message struct {
	Uuid      string         `json:"uuid"`
	Body      MessageBody    `json:"body"`
	Type      MessageType    `json:"type"`
	Command   MessageCommand `json:"command"`
	QueryArgs QueryArgument  `json:"queryArgs"`
	PullArgs  PullArgument   `json:"pullargs"`
}

type HistoryResult struct {
	Code     int      `json:"code"`
	Position int      `json:"position"`
	Message  string   `json:"msg"`
	Data     []string `json:"data"`
}
type Hub struct {
	// Registered clients.
	//clients map[*Client]bool

	// Inbound messages from the clients.

	ChanToBroadCast     chan MessageToSend
	ChanToSaveToLedis   chan MessageToSend
	ChanToSendServerACK chan MessageToSend
	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister   chan *Client
	groupClients map[string]GroupClients
}

func newHub() *Hub {
	return &Hub{
		ChanToBroadCast:     make(chan MessageToSend, 1024*8),
		ChanToSaveToLedis:   make(chan MessageToSend, 1024*8),
		ChanToSendServerACK: make(chan MessageToSend, 1024*8),
		register:            make(chan *Client, 8192),
		unregister:          make(chan *Client, 8192),
		groupClients:        make(map[string]GroupClients),
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func (h *Hub) ticker() {
	ticker := time.NewTicker(time.Second * 1)
	go func() {
		for t := range ticker.C {

			val := []byte(fmt.Sprintf("%d", t.Unix()))
			//fmt.Println("tick at",t)
			//f, er := os.OpenFile("/data/console.log", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0755)
			//if er != nil {
			//	f.Write([]byte(val));
			//	f.Close()
			//}

			h.ChanToBroadCast <- MessageToSend{groupName: "/timer", broadcast: val, client: nil}
		}
	}()
}
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			lock4Connections.Lock()
			_, ok := h.groupClients[client.groupName]
			if ok {
				h.groupClients[client.groupName][client] = true
			} else {
				groupClients := make(map[*Client]bool)
				groupClients[client] = true
				h.groupClients[client.groupName] = groupClients
			}
			lock4Connections.Unlock()

		case client := <-h.unregister:
			lock4Connections.Lock()
			_, ok := h.groupClients[client.groupName]
			if ok {
				delete(h.groupClients[client.groupName], client)
				//[client] = true
				close(client.send)
			}
			lock4Connections.Unlock()

		case message := <-h.ChanToBroadCast:
			//先看看group是否存在;
			_, ok := h.groupClients[message.groupName]
			if ok {
				for client := range h.groupClients[message.groupName] {
					if client == message.client {
						//就是发送方，那就啥也不干；
					} else {
						select {
						case client.send <- message.broadcast:
						default:
							close(client.send)
							delete(h.groupClients[message.groupName], client)
						}
					}
				}
			}
		case message := <-h.ChanToSendServerACK:
			select {
			case message.client.send <- message.broadcast:
			default:
				close(message.client.send)
				delete(h.groupClients[message.groupName], message.client)
			}
		case message := <-h.ChanToSaveToLedis:
			//将消息存入到ledis里;
			lcmd := redisClient.LPush(message.groupName, message.broadcast)
			/**
			如果推送失败入，就sleep一秒之后再推；
			还失败呢？
			那就不管了...
			*/
			if lcmd.Err() != nil {
				go func() {
					time.Sleep(1 * time.Second)
					secondPushCmd := redisClient.LPush(message.groupName, message.broadcast)
					if secondPushCmd.Err() != nil {
						log.Println("push to ledis/redis failed", message.groupName, string(message.broadcast))
					}
				}()
			}
		}
	}
}

func main() {
	flag.StringVar(&LedisAddr, "ledisAddr", ":6379", " address of ledis; 192.168.2.3:6379 etc")
	flag.StringVar(&secret, "secret", "Ilove9527", "the secret when you list all groups")
	flag.Parse()
	log.Println("...")
	hub := newHub()
	redisClient = redis.NewClient(&redis.Options{
		Addr:     LedisAddr,
		PoolSize: 128,
	})
	defer redisClient.Close()
	go hub.run()
	go hub.ticker()
	http.HandleFunc("/__/groups", func(w http.ResponseWriter, r *http.Request) {
		ServeGroups(hub, w, r)
	})
	http.HandleFunc("/__/history/", ServeHistoryMessage)
	http.HandleFunc("/__/ttl", serveTTL)
	http.HandleFunc("/__/cancel/", ServeCancel)
	http.HandleFunc("/__status", ServeStatus)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
