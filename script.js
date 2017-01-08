window.PeerConnection = window.RTCPeerConnection || window.mozRTCPeerConnection || window.webkitRTCPeerConnection;
window.RTCIceCandidate = window.RTCIceCandidate || window.mozRTCIceCandidate;
window.RTCSessionDescription = window.RTCSessionDescription || window.mozRTCSessionDescription;

var handle_ws_message = function(ws, e) {
  console.log("ws data:", e.data)
  var msg = JSON.parse(e.data);

  if (msg.event == "ping") {
    ws.send("pong");
  }

  if (msg.event == "sdp") {
    var sdp = msg.data;
    rtc.handleOffer(sdp);
  }

  if (msg.event == "ice") {
    // console.log("adding ice candidate");
    // var c = JSON.parse(msg.data);
    // var ice = new RTCIceCandidate(c);
    // rtc.pc.addIceCandidate(ice).then(function() {
    //   console.log("success");
    // }).catch(function(e) {
    //   console.log("error", e);
    // });
  }
}

function init() {
  var test = document.createTextNode('Hello world');
  document.body.appendChild(test);

  var ws = new WebSocket("ws://localhost:8080/ws");
  var rtc = new WebRTC(ws);
  window.ws = ws;
  window.rtc = rtc;

  ws.addEventListener('open', function(e) {
    console.log("connected", e);
  });
  ws.addEventListener('message', function(e) {
    handle_ws_message(ws, e);
  });
  ws.addEventListener('close', function(e) {
    console.log("disconnected", e);
  });
};

document.addEventListener('DOMContentLoaded', init);

var WebRTC = function(ws) {
  var config = {
    iceServers: [
      { urls: ["stun:stun.l.google.com:19302"] }
    ]
  };

  var pc = new PeerConnection(config, {
    optional: [
      { DtlsSrtpKeyAgreement: true },
    ],
  });

  pc.onconnectionstatechange = function(e) {
    console.log('connection state change', e);
  };

  pc.oniceconnectionstatechange = function(e) {
    console.log('ice connection state change', e);
  };

  pc.onicegatheringstatechange = function(e) {
    console.log("ice gathering state change", e);
  }

  pc.onicecandidate = function(evt) {
    console.log('got ice candidate', evt);
    var candidate = evt.candidate;
    if (candidate != null) {
      console.log("Got a candidate");
    } else {
      console.log("Finished gathering ICE candidates.");
      ws.send(JSON.stringify({event: 'answer', data: JSON.stringify(pc.localDescription)}))
      // Signalling.send(pc.localDescription);
      return;
    }
  };

  function prepareDataChannel(channel) {
    channel.onopen = function() {
      console.log("Data channel opened! (reliable: " + channel.reliable + ", ordered: " + channel.ordered +")");
    }
    channel.onclose = function() {
      console.log("Data channel closed.");
      console.log("------- chat disabled -------");
    }
    channel.onerror = function() {
      console.log("Data channel error!!");
    }
    channel.onmessage = function(msg) {
      var recv = msg.data;
      console.log(msg);
      // Go sends only raw bytes.
      if ("[object ArrayBuffer]" == recv.toString()) {
        var bytes = new Uint8Array(recv);
        line = String.fromCharCode.apply(null, bytes);
      } else {
        line = recv;
      }
      line = line.trim();
      console.log(line);
    }
  }

  pc.onnegotiationneeded = function() {
    console.log("negotiaYYYYtion needed?")
    console.log("negotiation needed?")
    console.log("negotiation needed?")
    console.log("nXXXXXXegotiation needed?")
    sendOffer();
  };

  pc.ondatachannel = function(dc) {
    console.log(dc);
    channel = dc.channel;
    console.log("Data Channel established... ");

    window.dc = dc;

    document.addEventListener('mousemove', function(e) {
      dc.channel.send(JSON.stringify({event: 'mousemove', data: JSON.stringify({x: e.clientX, y: e.clientY})}));
    });
    document.addEventListener('keydown', function(e) {
      dc.channel.send(JSON.stringify({event: 'mousemove', data: JSON.stringify({test: 1})}));
    });

    prepareDataChannel(channel);
  };

  return {
    handleOffer: function(offer) {
      var sdp = JSON.parse(offer).sdp;
      var desc = new RTCSessionDescription({
        type: 'offer',
        sdp: sdp
      });

      pc.setRemoteDescription(desc).then(function() {
        console.log("setting remote desc");
        pc.createAnswer(
          function(answer) {
            //success
            console.log("answer success", answer)
            pc.setLocalDescription(answer);
          },
          function(answer) {
            console.log("answer fail", answer)
          }
        );
      })
    },
    pc: pc
  }
};

