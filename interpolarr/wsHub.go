package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	pongWait   = 30 * time.Second    // Time allowed to read the next pong message from the peer
	pingPeriod = (pongWait * 9) / 10 // Ping period must be less than pongWait
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	logger     *logrus.Entry
	clients    map[*websocket.Conn]bool
	broadcast  chan interface{}
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	sync.Mutex
}

func NewHub() (*Hub, error) {
	logger, err := CreateLogger("ws")
	if err != nil {
		return nil, err
	}

	return &Hub{
		logger:     logger,
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan interface{}),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}, err
}

func (h *Hub) Run() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case conn := <-h.register:
			h.Lock()
			h.clients[conn] = true
			h.Unlock()
			h.logger.Debug("Client registered:", conn.RemoteAddr())

		case conn := <-h.unregister:
			h.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
				h.logger.Debug("Client unregistered:", conn.RemoteAddr())
			}
			h.Unlock()

		case message := <-h.broadcast:
			h.Lock()
			for conn := range h.clients {
				if err := conn.WriteJSON(message); err != nil {
					h.logger.Debugf("Error sending message to client %s: %v", conn.RemoteAddr(), err)
					go func(conn *websocket.Conn) { // Run the unregister in a separate goroutine (needed to not block)
						h.unregister <- conn
					}(conn)
				}
			}
			h.Unlock()

		case <-ticker.C:
			h.Lock()
			for conn := range h.clients {
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					go func(conn *websocket.Conn) { // Run the unregister in a separate goroutine (needed to not block)
						h.unregister <- conn
					}(conn)
				}
			}
			h.Unlock()
		}
	}
}

func (h *Hub) HandleConnections(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error(err)
		c.String(http.StatusInternalServerError, "Failed to upgrade to WebSocket")
		return
	}

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	h.register <- conn
}

func (h *Hub) BroadcastMessage(packet interface{}) {
	h.broadcast <- packet
}
