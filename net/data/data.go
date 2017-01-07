package data

import (
	"fmt"

	"golang.org/x/net/websocket"

	webrtc "github.com/keroserene/go-webrtc"
	"github.com/uber-go/zap"
)

var stun webrtc.ConfigurationOption

func init() {
	webrtc.SetLoggingVerbosity(3)
	stun = webrtc.OptionIceServer("stun:stun.l.google.com:19302")
}

// Conn encapsulates a WebRTC connection for a client
type Conn struct {
	pc *webrtc.PeerConnection
	dc *webrtc.DataChannel
	ws *websocket.Conn // Used for signalling
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

func (c *Conn) sendOffer(sdp string) {
	e := struct {
		Event string `json:"event"`
		Data  string `json:"data"`
	}{"sdp", sdp}

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
}

func (c *Conn) onIceComplete() {
	// w.log.Info("Finished Gathering ICE Candidates")
	fmt.Println("Finished gather ice candidates")
	sdp := c.pc.LocalDescription().Serialize()

	c.sendOffer(sdp)
	fmt.Println("sent offer")
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

// NewConn returns a new connection
func NewConn(signalws *websocket.Conn) *Conn {
	c := Conn{}
	c.ws = signalws

	config := webrtc.NewConfiguration(stun)
	pc, err := webrtc.NewPeerConnection(config)

	if nil != err {
		fmt.Println("Failed to create new peer connection", err)
		return nil
	}

	pc.OnNegotiationNeeded = c.onNegotiationNeeded
	pc.OnIceComplete = c.onIceComplete

	fmt.Println("Initializing Datachannel")
	opts := webrtc.Init{
		Ordered:        false,
		MaxRetransmits: 0,
	}
	dc, err := pc.CreateDataChannel("test", opts)
	if nil != err {
		fmt.Println("Failed to create channel", zap.Error(err))
		return nil
	}

	c.prepareDataChannel(dc)

	c.dc = dc
	c.pc = pc

	return &c
}
