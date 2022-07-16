"use strict";
//#const Websocket=require("ws")
exports.__esModule = true;
var gotapi_ws_1 = require("gotapi-ws");
var self = {
    lastItem: {
        aiMode: true,
        content: {
            type: 6
        }
    },
    aiMode: true,
    chat: {
        typing: true
    },
    chatItems: []
};
var wsHost = 'ws.gotapi.net';
var chatId = "hello-1";
var socketBus = new gotapi_ws_1.SocketBus("wss://".concat(wsHost).concat(gotapi_ws_1.WsConst.WsMsgChannel).concat(chatId)); // /chat/","http://localhost:4998/__/history/");
var messageBus = new gotapi_ws_1.MessageBus(gotapi_ws_1.Direction.Customer, socketBus, "https://".concat(wsHost, "/__/history/"), "".concat(gotapi_ws_1.WsConst.WsMsgChannel).concat(chatId), 100, 0);
messageBus.setNewMessageCallback(function (newMsg, uuid) {
    try {
        var event_1 = JSON.parse(newMsg.data);
        if (event_1.eventType !== gotapi_ws_1.WsEventType.ChatMessage) {
            console.log('只有ChatMessage才应该被广播到这个通道');
            return;
        }
        var body = event_1.data;
        body.uuid = uuid;
        console.log(newMsg);
        //console.log(body);
        var dayS = 60 * 60 * 24;
        var nowDate = new Date().getTime() / 1000;
        var date1 = nowDate - ((nowDate + 8 * 3600) % dayS) + dayS; // 明天凌晨
        // 处理时间格式
        if (body.direction === 'say' && body.content.type === 5) {
            var date2 = new Date(body.content.answer * 1000);
            var month = date2.getMonth() + 1;
            var day = date2.getDate();
            var hours = date2.getHours();
            var minutes = date2.getMinutes() < 10 ? "0".concat(date2.getMinutes()) : date2.getMinutes();
            var diff = (date1 - body.content.answer) / 3600;
            var printDate = void 0;
            if (diff < 24) {
                printDate = "\u4ECA\u5929".concat(hours, ":").concat(minutes);
            }
            else if (diff < 48) {
                printDate = "\u6628\u5929".concat(hours, ":").concat(minutes);
            }
            else {
                printDate = "".concat(month, "\u6708").concat(day, "\u65E5 ").concat(hours, ":").concat(minutes);
            }
            body.printDate = printDate;
        }
        // 处理最新消息
        if (body.direction === 'say' && body.content.type !== 5) {
            self.lastItem = body;
        }
        if (self.lastItem) {
            self.aiMode = self.lastItem.aiMode;
            if (self.lastItem.content.type === 6) {
                self.aiMode = true;
            }
            // if (self.lastItem.content.type == 1) {
            //   const menu = [{
            //     label: `<span style="color:#666;font-size:12px;">${self.lastItem.content.answer}</span>`,
            //     type: 'info',
            //   }];
            //   for (const x in self.lastItem.content.answer_pre) {
            //     menu.push({ label: self.lastItem.content.answer_pre[x] });
            //   }
            //   self.actionsheetMenu = menu;
            //   self.actionsheetShow = true;
            // }
        }
        self.chat.typing = false;
        //self.chatItems.push(body);
    }
    catch (e) {
        console.log(e);
    }
});
messageBus.setServerAckCallback(function () {
});
