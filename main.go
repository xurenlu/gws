package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/gernest/front"
	"github.com/gin-gonic/gin"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var lock4Connections sync.RWMutex

var addr = flag.String("addr", ":4998", "http service address")

var secret string
var yamlFormatter = front.NewMatter()

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
	Data interface{} `json:"data"`
	Sent bool        `json:"sent"`
	Hash string      `json:"hash"`
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
		}
	}
}

func GetMarkdownBody(content []byte) []byte {
	yamlFormatter.Handle("---", front.YAMLHandler)
	reader := bytes.NewReader(content)
	_, body, err := yamlFormatter.Parse(reader)
	if err != nil {
		return content
	}
	return []byte(body)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	flag.StringVar(&secret, "secret", "Ilove95271983", "the secret when you list all groups")
	flag.Parse()
	yamlFormatter.Handle("---", front.YAMLHandler)

	hub := newHub()

	go hub.run()
	go hub.ticker()

	router := gin.New()
	router.SetFuncMap(template.FuncMap{
		"safe": func(str string) template.HTML {
			return template.HTML(str)
		},
	})
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.html")
	router.Static("/static", "static")
	router.Any("/__status", ServeStatus)
	router.GET("/", func(context *gin.Context) {
		context.Redirect(302, "/docs/index/zh_cn")
	})
	router.GET("/docs/:page/:lang", func(c *gin.Context) {

		fileContent, er := ioutil.ReadFile("./md/gws.md")
		if er != nil {
			c.JSON(500, gin.H{})
			return
		}
		mdBytes := GetMarkdownBody(fileContent)
		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		mdparser := parser.NewWithExtensions(extensions)
		html := markdown.ToHTML(mdBytes, mdparser, nil)
		c.HTML(http.StatusOK, strings.ToLower(c.Param("lang"))+"."+strings.ToLower(c.Param("page"))+".html", gin.H{
			"Html": template.HTML(html)})
	})

	router.NoRoute(func(context *gin.Context) {
		if "POST" == context.Request.Method {
			servePost(hub, context)
		} else {
			serveGet(hub, context)
		}
	})
	err := router.Run(":" + port)
	if err != nil {
		return
	}

}
