package main

import (
	"bytes"
	"github.com/gorilla/websocket"
	"log"
	"strings"
	"time"
)

type Client struct {
	hub *Hub

	// The websocket connection.
	conn        *websocket.Conn
	JoinedGroup string
	// Buffered channel of outbound messages.
	send      chan []byte
	groupName string
}

// ReadPump
// pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		if c.groupName == "" || c.groupName == "/" {
			//@todo 这一块历史遗留问题。最早的版本是支持在连接上websocket后发一条join:{groupName}的方式来加入一个组的；因为老的业务系统还在线上运行，所以保留了。
			if strings.HasPrefix(string(bytes.TrimSpace(message)), "join:") {
				runes := []rune(string(message))
				start := len("join:")
				groupName := string(runes[start:len(string(message))])
				c.groupName = groupName
				c.hub.register <- c
			}
		} else {
			//message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
			if c.groupName != "/timer" {
				//并不接受从客户端发过来的名为timer的组的消息;
				//log.Println(string(message))
				handleNewWsData(c.hub, c, message, c.groupName)
			} else {
				log.Printf(c.groupName)
			}
		}

	}
}

// WritePump
//
//	pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:

			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(<-c.send)
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Println("write ping message error:", err)
				return
			}
		}
	}
}
