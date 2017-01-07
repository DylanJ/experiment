var handle_ws_message = function(ws, e) {
  if (e.data == "ping") {
    ws.send("pong");
  }
}

function init() {
  var test = document.createTextNode('Hello world');
  document.body.appendChild(test);

  var ws = new WebSocket("ws://localhost:8080/ws");
  ws.addEventListener('open', function(e) {
    console.log("connected", e);
  });
  ws.addEventListener('message', function(e) {
    handle_ws_message(ws, e);
  });
  window.ws = ws;
};

document.addEventListener('DOMContentLoaded', init);
