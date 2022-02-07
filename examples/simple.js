import {Direction, MessageBus, SocketBus} from "node-gotapi-ws";
const channel = "/note/shifen/me7pb5s6a3shifen.de"
//const wsHost = "ws.gotapi.net"
const wsHost = "localhost:4998"
let socketBus = new SocketBus(`ws://${wsHost}${channel}`); // /chat/","http://localhost:4998/__/history/");
let messageBus = new MessageBus(Direction.Customer, socketBus, `http://${wsHost}/__/history/`,
    `${channel}`, 100, 0)
messageBus.setNewMessageCallback(
    (newMsg, uuid) => {
        console.log("new msg arrived")
        console.log(newMsg.data)
        console.log(messageBus.sortedHash);
    }
);
setTimeout(()=>{

    console.log(messageBus.sortedHash);
    messageBus.pushMessage("god bless u");
    messageBus.direction
},1000)



