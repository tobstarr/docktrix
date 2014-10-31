var http = require("http");

var cnt = 0;
http.createServer(function(req, res) {
  cnt++;
  console.log("request", cnt);
  res.end("hello world");
}).listen(1234, "127.0.0.1");
