var isInBrowser = true;
let WebSocket = require("ws")
//let navigator;
var myws = new WebSocket("ws://localhost:4998/hello/baby");
// @ts-ignore
myws.on("close",function () {
    // @ts-ignore
    myws.connect();
});
