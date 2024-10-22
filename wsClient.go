package main

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 30 * time.Second    // Time allowed to read the next pong message from the peer
	pingPeriod = (pongWait * 9) / 10 // Ping period must be less than pongWait
)

type Client struct {
	hub  *Hub
	conn *websocket.Conn
}

func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
	}
}

func (c *Client) pingClient() {
	defer func() {
		c.hub.unregister <- c
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return // Connection is broken, close it
		}
	}
}
