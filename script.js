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
    StartWebRTC(ws, sdp)
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
  ws.addEventListener('close', function(e) {
    console.log("disconnected", e);
  });
  window.ws = ws;
};

document.addEventListener('DOMContentLoaded', init);

var StartWebRTC = function(ws, offer) {
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

  pc.onicecandidate = function(evt) {
    var candidate = evt.candidate;
    if (null == candidate) {
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
    sendOffer();
  };
  pc.ondatachannel = function(dc) {
    console.log(dc);
    channel = dc.channel;
    console.log("Data Channel established... ");

		window.dc = dc;

    prepareDataChannel(channel);
	};

	var sdp = JSON.parse(offer).sdp;
  var desc = new RTCSessionDescription({
    type: 'offer',
    sdp: sdp
  });

	pc.setRemoteDescription(desc).then(function() {
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
};

