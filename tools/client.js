const webSocket = require('ws');
let ws = new webSocket("ws://localhost:5002/hello")

ws.on('open', function open() {
    console.log("connected");
	setTimeout(function(){
		ws.send("hello")
	},200);
	
});
ws.on('message', function incoming(data) {
	console.log("got data:"+data);
    let user = JSON.parse(data);
    Mail.send("一个叫"+user.name+"的好心人支付了"+user.amount+"元，让主赞美他！");
});
