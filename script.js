function init() {
  var test = document.createTextNode('Hello world');
  document.body.appendChild(test);

  var ws = new WebSocket("ws://localhost:8080/ws");
  ws.addEventListener('open', function(e) {
    console.log("connected", e);
  });
  ws.addEventListener('message', function(e) {
    console.log("message", e);
  });
  window.ws = ws;
};

document.addEventListener('DOMContentLoaded', init);
