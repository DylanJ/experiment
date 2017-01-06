package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"golang.org/x/net/websocket"
)

type usercon struct {
	conn *websocket.Conn
}

// WS is a websocket thing
type WS struct {
}

type connections map[*websocket.Conn]bool

// Mux is a muxer
type Mux struct {
	ops   chan func(connections)
	users []usercon
}

// NewMux returns a new ws muxer
func NewMux() *Mux {
	m := Mux{}
	m.ops = make(chan func(connections))
	return &m
}

// Register a websocket
func (m *Mux) Register(conn *websocket.Conn) {

	result := make(chan int, 1)

	fmt.Println("start")
	m.ops <- func(m connections) {
		m[conn] = true
		fmt.Println("client registered:", conn.RemoteAddr().String())
		result <- 1
	}

	finished := <-result
	if finished == 1 {
		m.Send(conn, "foobar")
	}

	fmt.Println("fin")
}

// Unregister a websocket
func (m *Mux) Unregister(conn *websocket.Conn) {
	fmt.Println("unregistering client")
	m.ops <- func(m connections) {
		delete(m, conn)
	}
}

// ListClients hsow all slcients
func (m *Mux) ListClients() {
	fmt.Println("Listing Clients")
	m.ops <- func(m connections) {
		for addr := range m {
			fmt.Println("con: ", addr.RemoteAddr().String())
		}
	}
}

// ReadClient reads the meessages fomr athe fclinmetn
func (m *Mux) ReadClient(conn *websocket.Conn) {
	lastPong := time.Now()

	for {
		if lastPong.Add(time.Duration(time.Second * 2)).Before(time.Now()) {
			fmt.Println("sending ping")
			if m.sendPing(conn) {
				fmt.Println("keeping connection alive")
				lastPong = time.Now()
			} else {
				m.Unregister(conn)
				// ungraceful disconnect
				fmt.Println("closing connection")
				return
			}
		}

		var reply string
		conn.SetReadDeadline(time.Now().Add(time.Duration(1 * time.Second)))
		err := websocket.Message.Receive(conn, &reply)

		if err == io.EOF {
			// graceful disconncet
			fmt.Println("eof")
			m.Unregister(conn)
			return
		}

		if e, ok := err.(net.Error); ok && e.Timeout() {
			fmt.Println("timeout")
		}
	}
}

func (m *Mux) sendPing(conn *websocket.Conn) bool {
	deadline := time.Now().Add(time.Duration(2 * time.Second))
	conn.SetWriteDeadline(deadline)
	conn.SetReadDeadline(deadline)
	conn.Write([]byte("ping"))

	var reply string
	err := websocket.Message.Receive(conn, &reply)

	if err != nil {
		return false
	}

	if reply == "pong" {
		return true
	}

	return false
}

// Loop keeps things happening
func (m *Mux) Loop() {
	conns := make(connections)

	fmt.Println("q")
	for op := range m.ops {
		op(conns)
	}
}

// Send a message
func (m *Mux) Send(conn *websocket.Conn, msg string) error {
	result := make(chan bool, 1)
	m.ops <- func(m connections) {
		v := m[conn]
		result <- v
	}

	found := <-result
	if found == false {
		fmt.Printf("client '%s' not registered", conn.RemoteAddr().String())
		return errors.Errorf("client '%s' not registered", conn.RemoteAddr().String())
	}

	_, err := conn.Write([]byte(msg))
	return err
}

// App does all
type App struct {
	mux Mux
	ws  WS
}

func main() {
	mux := NewMux()
	go mux.Loop()

	http.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		mux.Register(ws)
		mux.ReadClient(ws)
	}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/script.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "script.js")
	})
	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "style.css")
	})

	http.ListenAndServe(":8080", nil)
}
