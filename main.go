package main

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/uber-go/zap"

	"golang.org/x/net/websocket"
)

// User represents a connection
type User struct {
	id   int
	conn *websocket.Conn
}

type connections map[*websocket.Conn]bool

// Mux is a muxer
type Mux struct {
	log zap.Logger
	ops chan func(connections)
}

// NewMux returns a new ws muxer
func NewMux(logger zap.Logger) *Mux {
	m := Mux{}
	m.ops = make(chan func(connections))
	m.log = logger
	return &m
}

// Register a websocket
func (m *Mux) Register(conn *websocket.Conn) {
	result := make(chan int, 1)

	m.ops <- func(m connections) {
		m[conn] = true
		result <- 1
	}

	finished := <-result
	if finished == 1 {
		m.log.Info(
			"Client Registered",
			zap.String("Address", conn.RemoteAddr().String()),
		)
		if err := m.Send(conn, "foobar"); err != nil {
			m.log.Info(
				"Client failed to register",
				zap.String("Address", conn.RemoteAddr().String()),
				zap.Error(err),
			)
		}
	}
}

// Unregister the client.
func (m *Mux) Unregister(conn *websocket.Conn) {
	m.log.Info(
		"Client Unregistered",
		zap.String("Address", conn.RemoteAddr().String()),
	)

	m.ops <- func(m connections) {
		delete(m, conn)
	}
}

// ReadClient reads the meessages fomr athe fclinmetn
func (m *Mux) ReadClient(conn *websocket.Conn) {
	lastPong := time.Now()

	for {
		if lastPong.Add(time.Duration(time.Second * 2)).Before(time.Now()) {
			m.log.Info(
				"Sending Ping",
				zap.String("address", conn.RemoteAddr().String()),
			)

			if m.sendPing(conn) {
				m.log.Info(
					"Client Keep-Alive",
					zap.String("address", conn.RemoteAddr().String()),
				)

				lastPong = time.Now()
			} else {
				m.Unregister(conn)
				return
			}
		}

		var reply string
		conn.SetReadDeadline(time.Now().Add(time.Duration(1 * time.Second)))
		err := websocket.Message.Receive(conn, &reply)

		if err == io.EOF {
			m.Unregister(conn)
			return
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
		return errors.Errorf("Client '%s' not registered", conn.RemoteAddr().String())
	}

	_, err := conn.Write([]byte(msg))
	return err
}

// App does all
type App struct {
	mux Mux
}

func main() {
	logger := zap.New(zap.NewTextEncoder(zap.TextNoTime()), zap.Output(os.Stdout))
	mux := NewMux(logger)
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
