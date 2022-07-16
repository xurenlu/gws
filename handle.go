// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 2 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 2 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 8192 * 4
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4196,
	WriteBufferSize: 4196,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func handleNewWsData(hub *Hub, clt *Client, message []byte, groupName string) {
	var messageObj Message

	if marshalErr := json.Unmarshal(message, &messageObj); marshalErr != nil {
		//解码不了，做一个广播;
		messageToSend := MessageToSend{groupName: groupName, broadcast: message, client: clt}
		hub.ChanToBroadCast <- messageToSend
	} else {
		if messageObj.Type == 0 && messageObj.Command == 0 {
			/**
			如果是0,表示并不是一个聊天消息，所以还是继续广播掉;
			*/
			messageToSend := MessageToSend{groupName: groupName, broadcast: message, client: clt}
			hub.ChanToBroadCast <- messageToSend
		} else if messageObj.Type == MessageTypeCommand {
			if messageObj.Command == MessageCommandPingPong {
				//ping pong,do nothing;
				//log.Println("pingpong")
				//break;
			}
		} else if messageObj.Type == MessageTypeData {

			messageToSend := MessageToSend{groupName: groupName, broadcast: message, client: clt}
			/**
			所有的消息类型，都做广播，但是，只有数据类型的，才做存储；同时做一个SERVER_ACK的回应;
			*/
			hub.ChanToBroadCast <- messageToSend
			hub.ChanToSaveToLedis <- messageToSend
			if clt == nil {
				return
			}
			//@todo 并构造一条SERVER_ACK消息,将之广播出去;
			serverAckMessage := Message{}
			serverAckMessage.Type = MessageTypeCommand
			serverAckMessage.Command = MessageCommandServerAck
			serverAckMessage.Uuid = messageObj.Uuid
			ackMessage, ackJsonError := json.Marshal(serverAckMessage)
			if ackJsonError != nil {
				log.Println("ackMessage generate failed")
			}
			commandAck := MessageToSend{groupName: groupName, client: clt, broadcast: ackMessage}
			hub.ChanToSendServerACK <- commandAck
		}
	}

}
