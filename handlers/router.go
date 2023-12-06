package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shinhagunn/websocket/config"
	"github.com/shinhagunn/websocket/pkg/routing"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin:     checkSameOrigin(os.Getenv("API_CORS_ORIGINS")),
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func wsHandler(epoll *routing.Epoll) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		conn.SetReadLimit(maxMessageSize)
		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		auth := routing.Auth{
			UID:  r.Header.Get("JwtUID"),
			Role: r.Header.Get("JwtRole"),
		}

		client := routing.NewClient(conn, auth)

		if err := epoll.Add(client); err != nil {
			log.Printf("Failed to add connection %v", err)
			conn.Close()
		}
	}
}

func SetupRoutes(config *config.Config, epoll *routing.Epoll) error {
	// pub, err := getPublicKey(config)
	// if err != nil {
	// 	return errors.Wrap(err, "Loading public key failed")
	// }

	http.HandleFunc("/", authHandler(wsHandler(epoll), nil, false))
	http.HandleFunc("/public", authHandler(wsHandler(epoll), nil, false))
	http.HandleFunc("/private", authHandler(wsHandler(epoll), nil, true))

	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		return err
	}

	return nil
}
