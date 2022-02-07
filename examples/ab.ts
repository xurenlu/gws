let isInBrowser = true;
//let navigator;

let myws = new WebSocket("ws://localhost:4998/hello/baby");
// @ts-ignore
myws.onclose(()=>{
    // @ts-ignore
    myws.connect();
})