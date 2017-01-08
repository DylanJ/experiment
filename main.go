package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/dylanj/bombs/net/data"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/uber-go/zap"

	"golang.org/x/net/websocket"
)

var err error

// User represents a connection
type User struct {
	id   string
	conn *websocket.Conn
	data *data.Conn
	buf  []string
}

// FooBar a message to user
func (u *User) FooBar() {
	u.buf = make([]string, 32)
}

// SendData a message to user
func (u *User) SendData(data string) {
	u.buf = append(u.buf, data)
}

// DrainData ad
func (u *User) DrainData() {
	fmt.Println("draining user data")
	if len(u.buf) == 0 {
		fmt.Println("no messages to drain")
		return
	}

	for _, msg := range u.buf {
		if len(msg) > 0 {
			u.data.Send(msg)
		}
	}

	u.buf = make([]string, 32)
}

// Event is a wrapper object for a message sent over websockets
type Event struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

type connections map[*websocket.Conn]*User

// Mux is a muxer
type Mux struct {
	log   zap.Logger
	ops   chan func(connections)
	users []*User
}

// NewMux returns a new ws muxer
func NewMux(logger zap.Logger) *Mux {
	m := Mux{}
	m.ops = make(chan func(connections))
	m.log = logger
	m.users = make([]*User, 32)
	return &m
}

// Register a websocket
func (m *Mux) Register(u *User) {
	result := make(chan int, 1)

	m.users = append(m.users, u)
	m.ops <- func(m connections) {
		m[u.conn] = u
		result <- 1
	}

	finished := <-result
	if finished == 1 {
		m.log.Info(
			"Client Registered",
			zap.String("Address", u.conn.LocalAddr().String()),
		)
	}
}

// DrainData sends data to all people
func (m *Mux) DrainData() {
	fmt.Println("STARTxawtf")
	for _, u := range m.users {
		if u == nil {
			continue
		}
		fmt.Println("wtf")
		u.DrainData()
	}
	fmt.Println("FINxawtf")
}

// DataBroadcast sends data to all clients
func (m *Mux) DataBroadcast(data string) {
	m.ops <- func(m connections) {
		for conn := range m {
			fmt.Println("start:", conn)
			u := m[conn]
			fmt.Println("halfway")
			u.SendData(data)
			// u.data.Send(data)
			fmt.Println("finish:")
		}
	}
}

// Unregister the client.
func (m *Mux) Unregister(conn *websocket.Conn) {
	m.log.Info(
		"Client Unregistered",
		zap.String("Address", conn.LocalAddr().String()),
	)

	m.ops <- func(m connections) {
		delete(m, conn)
	}
}

// ReadClient reads the meessages fomr athe fclinmetn
func (m *Mux) ReadClient(u *User) {
	lastPong := time.Now()
	addr := u.conn.Request().RemoteAddr

	for {
		if lastPong.Add(time.Duration(time.Second * 2)).Before(time.Now()) {
			m.log.Info(
				"Sending Ping",
				zap.String("address", addr),
			)

			if m.sendPing(u.conn) {
				m.log.Info(
					"Client Keep-Alive",
					zap.String("address", addr),
				)

				lastPong = time.Now()
			} else {
				m.Unregister(u.conn)
				return
			}
		}

		var reply Event
		u.conn.SetReadDeadline(time.Now().Add(time.Duration(1 * time.Second)))
		err := websocket.JSON.Receive(u.conn, &reply)

		if err == io.EOF {
			m.Unregister(u.conn)
			return
		}

		if reply.Event == "answer" {
			u.data.ReceiveAnswer(reply.Data)
		}
	}
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
		fmt.Println("loop st")
		op(conns)
		fmt.Println("loop end")
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
	ticker := time.NewTicker(100 * time.Millisecond)
	quit := make(chan struct{})
	go func() {
		for {
			fmt.Println("di")
			select {
			case <-ticker.C:
				fmt.Println("draining")
				mux.DrainData()
				fmt.Println("finished draining")
				break
			case <-quit:
				fmt.Println("quiting")
				ticker.Stop()
				return
			}
		}
	}()

	http.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {

		dc := data.NewConn(
			ws,
			func(msg []byte) {
				var e Event

				json.Unmarshal(msg, &e)
				fmt.Println("event", e.Event, "data:", e.Data)

				if e.Event == "mousemove" {
					mux.DataBroadcast(e.Data)
				}

			},
		)

		u := User{
			conn: ws,
			data: dc,
			id:   uuid.NewV4().String(),
		}

		u.FooBar()

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
