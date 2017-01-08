package data

import (
	"fmt"

	"golang.org/x/net/websocket"

	webrtc "github.com/keroserene/go-webrtc"
	"github.com/uber-go/zap"
)

var stun webrtc.ConfigurationOption

func init() {
	webrtc.SetLoggingVerbosity(1)
	stun = webrtc.OptionIceServer("stun:stun.l.google.com:19302")
}

// Conn encapsulates a WebRTC connection for a client
type Conn struct {
	pc *webrtc.PeerConnection
	dc *webrtc.DataChannel
	ws *websocket.Conn // Used for signalling
}

// Send sends data
func (c *Conn) Send(data string) {
	if len(data) == 0 {
		fmt.Println("dont send nothing")
		return
	}
	c.dc.Send([]byte(data))
}

// ReceiveAnswer is for foo
func (c *Conn) ReceiveAnswer(answer string) {
	sdp := webrtc.DeserializeSessionDescription(answer)
	if nil == sdp {
		fmt.Println("Invalid SDP.")
		return
	}
	err := c.pc.SetRemoteDescription(sdp)
	if nil != err {
		fmt.Println("ERROR", err)
		return
	}
}

func (c *Conn) sendWS(event string, sdp string) {
	e := struct {
		Event string `json:"event"`
		Data  string `json:"data"`
	}{event, sdp}

	websocket.JSON.Send(c.ws, e)
}

func (c *Conn) onNegotiationNeeded() {
	fmt.Println("negotiation needed")
	go c.generateOffer()
}

func (c *Conn) generateOffer() {
	offer, err := c.pc.CreateOffer() // blocks
	if nil != err {
		fmt.Println("offer err:", err)
	}

	c.pc.SetLocalDescription(offer)
	// sdp := c.pc.LocalDescription().Serialize()

	// c.sendOffer(sdp)
}

func (c *Conn) onIceComplete() {
	// w.log.Info("Finished Gathering ICE Candidates")
	fmt.Println("Finished gather ice candidates")
	sdp := c.pc.LocalDescription().Serialize()

	c.sendWS("sdp", sdp)
	fmt.Println("sent offer")
}

func (c *Conn) onIceCandidate(ice webrtc.IceCandidate) {
	// send the candidate to the client
	c.sendWS("ice", ice.Serialize())
}

func (c *Conn) prepareDataChannel(channel *webrtc.DataChannel) {
	channel.OnOpen = func() {
		fmt.Println("Data Opening Channel")
		fmt.Println("(ordered:", channel.Ordered(), ", MaxRetransmits:", channel.MaxRetransmits())
		fmt.Println("(MaxLife", channel.MaxPacketLifeTime())
	}

	channel.OnClose = func() {
		fmt.Println("Data Channel Closed")
	}

	channel.OnMessage = func(msg []byte) {
		fmt.Println("Data Channel message", "msg", string(msg))
		channel.Send([]byte("hey!"))
	}
}

func (c *Conn) onDataMessage(msg []byte) {
	fmt.Println("msg: ", string(msg))
}

// NewConn returns a new connection
func NewConn(signalws *websocket.Conn, onMessage func([]byte)) *Conn {
	c := Conn{}
	c.ws = signalws

	config := webrtc.NewConfiguration(stun)
	pc, err := webrtc.NewPeerConnection(config)
	c.pc = pc

	if nil != err {
		fmt.Println("Failed to create new peer connection", err)
		return nil
	}

	pc.OnNegotiationNeeded = c.onNegotiationNeeded
	pc.OnIceComplete = c.onIceComplete
	pc.OnIceCandidate = c.onIceCandidate
	pc.OnIceCandidateError = func() {
		fmt.Println("ICE candidate error")
	}
	pc.OnDataChannel = func(dc *webrtc.DataChannel) {
		fmt.Println("Datachannel GOGO")
	}

	fmt.Println("Initializing Datachannel")
	dc, err := pc.CreateDataChannel(
		"test",
		webrtc.Ordered(false),
		webrtc.MaxRetransmits(0),
	)
	if nil != err {
		fmt.Println("Failed to create channel", zap.Error(err))
		return nil
	}

	dc.OnOpen = func() {
		fmt.Println("Data Opening Channel")
		fmt.Println("(ordered:", dc.Ordered(), ", MaxRetransmits:", dc.MaxRetransmits())
		fmt.Println("(MaxLife", dc.MaxPacketLifeTime())
	}

	dc.OnClose = func() {
		fmt.Println("Data Channel Closed")
	}
	dc.OnMessage = onMessage

	c.dc = dc

	return &c
}
