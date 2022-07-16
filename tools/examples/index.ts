//#const Websocket=require("ws")


import {
	Direction,
	SocketBus,
	MessageBus,
	WsConst,
	WsEventType,
} from 'gotapi-ws';
const self = {
	lastItem: {
		aiMode: true,
		content: {
			type: 6
		}
	},
	aiMode: true,
	chat: {
		typing: true,
	},
	chatItems: []
};
const wsHost = 'ws.gotapi.net';
const chatId = "hello-1";
let socketBus = new SocketBus(`wss://${wsHost}${WsConst.WsMsgChannel}${chatId}`); // /chat/","http://localhost:4998/__/history/");
let messageBus = new MessageBus(Direction.Customer, socketBus, `https://${wsHost}/__/history/`, `${WsConst.WsMsgChannel}${chatId}`, 100, 0)
messageBus.setNewMessageCallback(
	(newMsg, uuid) => {
		try {
			console.log("data:")
			console.log(newMsg)
			const event = JSON.parse(newMsg.data);
			if (event.eventType !== WsEventType.ChatMessage) {
				console.log('只有ChatMessage才应该被广播到这个通道');
				return;
			}
			const body = event.data;
			body.uuid = uuid;
			console.log(newMsg);

			//console.log(body);
			const dayS = 60 * 60 * 24;
			const nowDate = new Date().getTime() / 1000;
			const date1 = nowDate - ((nowDate + 8 * 3600) % dayS) + dayS; // 明天凌晨
			// 处理时间格式
			if (body.direction === 'say' && body.content.type === 5) {
				const date2 = new Date(body.content.answer * 1000);
				const month = date2.getMonth() + 1;
				const day = date2.getDate();
				const hours = date2.getHours();
				const minutes = date2.getMinutes() < 10 ? `0${date2.getMinutes()}` : date2.getMinutes();
				const diff = (date1 - body.content.answer) / 3600;
				let printDate;
				if (diff < 24) {
					printDate = `今天${hours}:${minutes}`;
				} else if (diff < 48) {
					printDate = `昨天${hours}:${minutes}`
				} else {
					printDate = `${month}月${day}日 ${hours}:${minutes}`
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
		} catch (e) {
			console.log(e);
		}
	}
);
messageBus.setServerAckCallback(() => {

});
setTimeout(()=>{
	messageBus.pushMessage("Hello baby");
},1000)


