package main

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/dylanj/bombs/net/data"
	"github.com/pkg/errors"
	"github.com/uber-go/zap"

	"golang.org/x/net/websocket"
)

var err error

// User represents a connection
type User struct {
	id   int
	conn *websocket.Conn
	data *data.Conn
}

type connections map[*websocket.Conn]*User

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
func (m *Mux) Register(u *User) {
	result := make(chan int, 1)

	m.ops <- func(m connections) {
		m[u.conn] = u
		result <- 1
	}

	finished := <-result
	if finished == 1 {
		m.log.Info(
			"Client Registered",
			zap.String("Address", u.conn.RemoteAddr().String()),
		)
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
func (m *Mux) ReadClient(u *User) {
	lastPong := time.Now()

	for {
		if lastPong.Add(time.Duration(time.Second * 2)).Before(time.Now()) {
			m.log.Info(
				"Sending Ping",
				zap.String("address", u.conn.RemoteAddr().String()),
			)

			if m.sendPing(u.conn) {
				m.log.Info(
					"Client Keep-Alive",
					zap.String("address", u.conn.RemoteAddr().String()),
				)

				lastPong = time.Now()
			} else {
				m.Unregister(u.conn)
				return
			}
		}

		type data struct {
			Event string
			Data  string
		}

		var reply data
		u.conn.SetReadDeadline(time.Now().Add(time.Duration(1 * time.Second)))
		// err := websocket.Message.Receive(conn, &reply)
		err := websocket.JSON.Receive(u.conn, &reply)

		if reply.Event == "answer" {
			u.data.ReceiveAnswer(reply.Data)
		}

		if err == io.EOF {
			m.Unregister(u.conn)
			return
		}
	}
}

// Event is a wrapper object for a message sent over websockets
type Event struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

func (m *Mux) sendPing(conn *websocket.Conn) bool {
	deadline := time.Now().Add(time.Duration(2 * time.Second))
	conn.SetWriteDeadline(deadline)
	conn.SetReadDeadline(deadline)
	websocket.JSON.Send(conn, Event{"ping", "foo"})

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
	result := make(chan *User, 1)
	m.ops <- func(m connections) {
		v := m[conn]
		result <- v
	}

	found := <-result
	if found == nil {
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
		dc := data.NewConn(ws)

		u := User{
			conn: ws,
			data: dc,
		}

		mux.Register(&u)
		mux.ReadClient(&u)
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
