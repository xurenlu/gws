---
title: Gws的架构设计
date: 2022-01-25 00:12:20
---

# gotapi-ws 的架构设计 

gotapi-ws 是一整套websocket im 实现,在最基础的websocket链接之上，实现了基于redis 的list的一个历史聊天记录的存档，以及消息撤回机制；其服务端由golang实现，客户端当然是brower端和nodejs端的实现。
现在在https://ws.404.ms 上已经部署了一套服务端，大家可以直接使用。（在ws.404.ms上http的链接已经重定向到Https了，所以请使用https和wss）。

## groupName的概念
GroupName 可以简单的理解为房间，连接时配置不同的groupName，就是不在同的房间聊天。
怎么配置不同的房间呢？
很简单，连接服务端时，websocket的地址的URL PATH部分就是groupName；如下面这个:

```javascript
const channel = "/note/shifen/me7pb5s6a3shifen.de"
const wsHost = "ws.404.ms"
let socketBus = new SocketBus(`wss://${wsHost}${channel}`);
```
*/note/shifen/me7pb5s6a3shifen.de* 就是groupName。

## server-side 支持两个层级的操作

- low level : 就是最基础的websocket操作，服务端收到数据后，即将数据广播出去。
- high level: 如果数据是被json编码的，且编码后识别是一个数据类型的包，则数据会被存进redis,再广播出去；存入redis的数据可以通过历史数据接口取回，也可以按uuid进行撤回。
low-level 和high-level的区别就是,high-level是按gotapi指定的格式封好的包，支持ack确认，支持历史消息，支持撤回。

## high-level的消息包的结构

一条high-level的典型的数据类型的消息如下:
```javascript
{
    "body":{
        "sent":false,
        "data":"hello world",
        "hash":"eb2ef7838a7a57d"
    },
    "type":2, //2表示是一条数据；1表示是一条命令。
    "uuid":"02108001177667145695",
}
```
一条high-level的典型的命令类型的消息如下:
```javascript
{
    "type":1,
    "command":1 //1表示是一条server side的ack消息，5表示是一条pingpong消息；
}
```

### nodejs端的 API

下面是一个简单的使用node-gotapi-ws包的示例,演示了如何利用gotapi-ws的high-level api进行一个简单的聊天系统：

```javascript
import {Direction, MessageBus, SocketBus} from "node-gotapi-ws";
const channel = "/note/shifen/me7pb5s6a3shifen.de"
const wsHost = "ws.404.ms"
let socketBus = new SocketBus(`wss://${wsHost}${channel}`); 
let messageBus = new MessageBus(Direction.Customer, socketBus, `https://${wsHost}/__/history/`,
    `${channel}`, 100, 0)
//设置一个回调，在有新消息到达时，会被触发。
messageBus.setNewMessageCallback(
    (newMsg, uuid) => {
        console.log("new msg arrived")
        console.log(newMsg.data)    
    }
);
setTimeout(()=>{
    //这里演示如何发送一条消息出去；发送消息有两种方式，这里是用的websocket链接来发送的，另一种方式是用http api来发送；
    messageBus.pushMessage("god bless u");
},1000)
```

## 使用http api来发消息

发送一条消息，可以使用websocket链接，也可以使用http 接口来发送；不论是low-level还是high-level,都支持用这两种方式来发送；下面演示一下如何用http 接口来发消息:

```bash
 curl -X POST -d "{\"type\":2,\"uuid\":\"thisisuuid\",\"body\":{\"hash\":\"hello1234\",\"data\":\"hello world\",\"sent\":false}}" https://ws.404.ms/room/9527/
```

虽然在websocket链接里可以发消息，但是我们推荐用http api来发消息。在以往的实践中，我们是在应用系统中通过http api 往这个聊天服务器发消息的，因为我们还要实现额外的会话存档、检索、合规等业务。

## 使用http api来拉取历史消息

  > 提醒:历史消息只针对high-level的消息有用；

可以按这样访问/__/history/来调取历史消息:
```bash
curl -X POST -d "groupName=/room/9527/&position=-50&size=50" https://ws.404.ms/__/history
```

在gotapi-ws node-gotapi-ws 的npm包里，已经处理好了，在创建messageBus时，有一个参数是指定history 大小的,MessageBus的构造函数如下:

```typescript
class MessageBus {
    constructor(direction: Direction, socketBus: SocketBus, historyUrl: string, groupName: string, historySize: number, position:number){
        ...
        if(self.historySize>0){                
            self.pullHistory();
        }
    }
}
```
这里的historySize，是指定当聊天在初始化的时候，从服务器上拉回多少条历史消息；

## 消息的撤回

golang 服务器提供了一个撤回接口:

```bash
curl -X POST -d "groupName=/room/9527/&uuid=some-uuid" https://ws.404.ms/__/cancel
```
撤回时，提交两个参数，一个是groupName,一个是uuid；groupName前文解释过；uuid是每条消息的唯一辨识码。



