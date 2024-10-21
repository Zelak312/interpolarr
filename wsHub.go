package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	logger     *logrus.Entry
	clients    map[*Client]bool
	broadcast  chan interface{}
	register   chan *Client
	unregister chan *Client
	sync.Mutex
}

func NewHub() (*Hub, error) {
	logger, err := CreateLogger("ws")
	if err != nil {
		return nil, err
	}

	return &Hub{
		logger:     logger,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan interface{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}, err
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.Lock()
			h.clients[client] = true
			h.Unlock()
			h.logger.Debug("Client registered:", client.conn.RemoteAddr())

		case client := <-h.unregister:
			h.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.conn.Close()
				h.logger.Debug("Client unregistered:", client.conn.RemoteAddr())
			}
			h.Unlock()

		case message := <-h.broadcast:
			h.Lock()
			for client := range h.clients {
				if err := client.conn.WriteJSON(message); err != nil {
					h.logger.Debugf("Error sending message to client %s: %v", client.conn.RemoteAddr(), err)
					h.unregister <- client
				}
			}
			h.Unlock()

		default:
			time.Sleep(100 * time.Millisecond)
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

	client := NewClient(h, conn)
	h.register <- client

	go client.pingClient()
}
