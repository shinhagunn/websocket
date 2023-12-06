package routing

import (
	"log"

	"github.com/gorilla/websocket"
)

type Auth struct {
	UID  string
	Role string
}

type Client struct {
	Auth Auth

	pubSub  []string
	privSub []string

	conn *websocket.Conn
}

// NewClient handles websocket requests from the peer.
func NewClient(conn *websocket.Conn, auth Auth) *Client {
	client := &Client{
		conn:    conn,
		Auth:    auth,
		pubSub:  []string{},
		privSub: []string{},
	}

	if client.Auth.UID == "" {
		log.Println("New anonymous connection")
	} else {
		log.Printf("New authenticated connection: %s\n", client.Auth.UID)
	}

	return client
}

func (c *Client) ReadMessage() (messageType int, p []byte, err error) {
	return c.conn.ReadMessage()
}

func (c *Client) WriteMessage(messageType int, data []byte) error {
	return c.conn.WriteMessage(messageType, data)
}

func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) Send(messageType int, data []byte) error {
	return c.WriteMessage(messageType, data)
}

func (c *Client) GetAuth() Auth {
	return c.Auth
}

func (c *Client) GetSubscriptions() []string {
	return append(c.pubSub, c.privSub...)
}

func (c *Client) SubscribePublic(s string) {
	if !contains(c.pubSub, s) {
		c.pubSub = append(c.pubSub, s)
	}
}

func (c *Client) SubscribePrivate(s string) {
	if !contains(c.privSub, s) {
		c.privSub = append(c.privSub, s)
	}
}

func (c *Client) UnsubscribePublic(s string) {
	l := make([]string, len(c.pubSub)-1)
	i := 0
	for _, el := range c.pubSub {
		if s != el {
			l[i] = el
			i++
		}
	}
	c.pubSub = l
}

func (c *Client) UnsubscribePrivate(s string) {
	l := make([]string, len(c.privSub)-1)
	i := 0
	for _, el := range c.privSub {
		if s != el {
			l[i] = el
			i++
		}
	}
	c.privSub = l
}
