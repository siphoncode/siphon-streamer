package streamer

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message
	pongWait = 60 * time.Second

	// Send pings with this period (must be less that pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed
	maxMessageSize = 1024 * 25

	messageRate = 20.0

	messagePer = 5.0
)

// Configure the upgrader that handles the websocket handshake
var upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	CheckOrigin:      func(r *http.Request) bool { return true },
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
}

// connection mediates between a websocket connection and a dispatcher.
type connection struct {
	connType    string
	userID      string
	appID       string
	ws          *websocket.Conn
	sendChannel chan []byte
}

// Logs a message prefixed with the remote host's address.
func (c *connection) log(s string, args ...interface{}) {
	log.Printf("[%s] %s", c.ws.RemoteAddr(), fmt.Sprintf(s, args...))
}

// Note that all connections are essentially bi-directional since pings
// need to be sent and pongs need to be received.
func (c *connection) write(msgType int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(msgType, payload)
}

// writePump pumps messages from the dispatcher to the websocket connection
func (c *connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	// Note that websocket.CloseMessage etc are constants that correspond to
	// ints in RFC 6455
	for {
		select {
		case msg, ok := <-c.sendChannel:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
			}
			if err := c.write(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection to the dispatcher
func (c *connection) readPump() {
	defer func() {
		d.unregister <- c
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline((time.Now().Add(pongWait)))
		return nil
	})

	// For throttling
	allowance := messageRate
	allowanceCheck := time.Now()
	allowanceReachedNotif := false

	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			if _, ok := err.(*websocket.CloseError); ok {
				c.log("Connection closed")
			} else {
				c.log("Error reading message: %s", err.Error())
			}
			break
		}
		if len(msg) != 0 {
			// Apply rate limiting (token bucket algorithm)
			currTime := time.Now()
			timeSinceLast := currTime.Sub(allowanceCheck)
			allowanceCheck = currTime
			allowance = allowance + (timeSinceLast.Seconds() * (messageRate / messagePer))
			if allowance > messageRate {
				allowance = messageRate
			}
			if allowance < 1 {
				if !allowanceReachedNotif {
					msgString := "[WARNING] Logging too frequent"
					allowanceReachedNotif = true
					msg = []byte(msgString)
				} else {
					continue // Don't broadcast this message
				}
				continue
			} else {
				allowanceReachedNotif = false
				allowance = allowance - 1
			}
			// Store it in message data structure
			m := message{
				appID:   c.appID,
				userID:  c.userID,
				payload: msg,
			}

			// Send the message down the appropriate channel (only connections
			// with connType notifications or logReader can receive messages)
			switch c.connType {
			case "notifications":
				d.broadcastNotification <- m
			case "log_writer":
				d.broadcastLog <- m
			}
		}
	}
}

// WebsocketHandler handles incoming requests for websocket connections
func websocketHandler(w http.ResponseWriter, r *http.Request) {
	// Get the app token from the url
	//fmt.Fprintf(w, "WebsocketHandler handling")
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	fmt.Printf("Decrypting Token...")

	tkn, connType, err := authorizedRequest(r)
	if err != nil {
		log.Println(err)
		http.Error(w, "Bad request", 400)
		return
	}

	// Upgrade the connection to a ws
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	c := &connection{
		connType:    connType,
		userID:      tkn.UserID,
		appID:       tkn.AppID,
		ws:          ws,
		sendChannel: make(chan []byte, maxMessageSize),
	}

	// Register the conection with the dispatcher
	d.register <- c
	go c.writePump()
	c.readPump()
}
