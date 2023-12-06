package routing

import (
	"bytes"
	"errors"
	"sync"
	"syscall"
	"time"

	"log"

	"github.com/gorilla/websocket"
	msgPkg "github.com/shinhagunn/websocket/pkg/message"

	"golang.org/x/sys/unix"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	maxBufferedMessages = 256
)

type Request struct {
	client *Client
	msgPkg.Request
}

type sendMessage struct {
	client *Client
	msg    []byte
}

type Epoll struct {
	fd          int
	connections map[int]*Client
	sendMessage chan sendMessage
	lock        *sync.RWMutex

	// List of clients registered to public topics
	PublicTopics map[string]*Topic

	// List of clients registered to private topics
	PrivateTopics map[string]map[string]*Topic
}

func MkEpoll() (*Epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	return &Epoll{
		fd:            fd,
		lock:          &sync.RWMutex{},
		sendMessage:   make(chan sendMessage, maxBufferedMessages),
		connections:   make(map[int]*Client),
		PublicTopics:  make(map[string]*Topic),
		PrivateTopics: make(map[string]map[string]*Topic),
	}, nil
}

func (e *Epoll) Add(client *Client) error {
	fd := websocketFD(client.conn)

	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Events: unix.POLLIN | unix.POLLHUP, Fd: int32(fd)})
	if err != nil {
		return err
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	e.connections[fd] = client
	if len(e.connections)%100 == 0 {
		log.Printf("Total number of connections: %v\n", len(e.connections))
	}

	return nil
}

func (e *Epoll) Remove(client *Client) error {
	fd := websocketFD(client.conn)

	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	delete(e.connections, fd)
	if len(e.connections)%100 == 0 {
		log.Printf("Total number of connections: %v\n", len(e.connections))
	}

	return nil
}

func (e *Epoll) Wait() ([]*Client, error) {
	events := make([]unix.EpollEvent, 100)
	n, err := unix.EpollWait(e.fd, events, 100)
	if err != nil {
		return nil, err
	}

	e.lock.RLock()
	defer e.lock.RUnlock()

	var connections []*Client
	for i := 0; i < n; i++ {
		client := e.connections[int(events[i].Fd)]
		connections = append(connections, client)
	}

	return connections, nil
}

func (e *Epoll) handleRequest(req *Request) {
	switch req.Method {
	case "subscribe":
		e.handleSubscribe(req)
	case "unsubscribe":
		e.handleUnsubscribe(req)
	default:
		e.sendMessage <- sendMessage{
			client: req.client,
			msg:    []byte(responseMust(errors.New("unsupported method"), nil)),
		}
	}
}

func (e *Epoll) handleSubscribe(req *Request) {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, t := range req.Streams {
		switch {
		case isPrivateStream(t):
			e.subscribePrivate(t, req)
		default:
			e.subscribePublic(t, req)
		}
	}

	e.sendMessage <- sendMessage{
		client: req.client,
		msg: []byte(responseMust(nil, map[string]interface{}{
			"message": "subscribed",
			"streams": req.client.GetSubscriptions(),
		})),
	}
}

func (e *Epoll) handleUnsubscribe(req *Request) {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, t := range req.Streams {
		switch {
		case isPrivateStream(t):
			e.unsubscribePrivate(t, req)
		// case isPrefixedStream(t):
		// 	h.unsubscribePrefixed(t, req)
		default:
			e.unsubscribePublic(t, req)
		}
	}

	e.sendMessage <- sendMessage{
		client: req.client,
		msg: []byte(responseMust(nil, map[string]interface{}{
			"message": "unsubscribed",
			"streams": req.client.GetSubscriptions(),
		})),
	}
}

func (e *Epoll) subscribePublic(t string, req *Request) {
	topic, ok := e.PublicTopics[t]
	if !ok {
		topic = NewTopic()
		e.PublicTopics[t] = topic
	}

	if topic.subscribe(req.client) {
		req.client.SubscribePublic(t)
	}
}

func (e *Epoll) subscribePrivate(t string, req *Request) {
	uid := req.client.GetAuth().UID
	if uid == "" {
		log.Printf("Anonymous user tried to subscribe to private stream %s\n", t)
		return
	}

	uTopics, ok := e.PrivateTopics[uid]
	if !ok {
		uTopics = make(map[string]*Topic, 3)
		e.PrivateTopics[uid] = uTopics
	}

	topic, ok := uTopics[t]
	if !ok {
		topic = NewTopic()
		uTopics[t] = topic
	}

	if topic.subscribe(req.client) {
		req.client.SubscribePrivate(t)
	}
}

func (e *Epoll) unsubscribePublic(t string, req *Request) {
	topic, ok := e.PublicTopics[t]
	if ok {
		if topic.unsubscribe(req.client) {
			req.client.UnsubscribePublic(t)
		}

		if topic.len() == 0 {
			delete(e.PublicTopics, t)
		}
	}
}

func (e *Epoll) unsubscribePrivate(t string, req *Request) {
	uid := req.client.GetAuth().UID
	if uid == "" {
		return
	}

	uTopics, ok := e.PrivateTopics[uid]
	if !ok {
		return
	}

	topic, ok := uTopics[t]
	if ok {
		if topic.unsubscribe(req.client) {
			req.client.UnsubscribePrivate(t)
		}

		if topic.len() == 0 {
			delete(uTopics, t)
		}
	}

	uTopics, ok = e.PrivateTopics[uid]
	if ok && len(uTopics) == 0 {
		delete(e.PrivateTopics, uid)
	}
}

func (e *Epoll) Read() {
	for {
		connections, err := e.Wait()
		if err != nil {
			log.Printf("Failed to epoll wait %v\n", err)
			continue
		}

		for _, client := range connections {
			if client == nil {
				break
			}

			_, mess, err := client.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("error: %v", err)
				}

				if err := e.Remove(client); err != nil {
					log.Printf("Failed to remove %v\n", err)
				}
				client.Close()
				continue
			}

			mess = bytes.TrimSpace(bytes.Replace(mess, []byte{'\n'}, []byte{' '}, -1))
			if len(mess) == 0 {
				continue
			}
			log.Printf("Received message %s\n", mess)

			if string(mess) == "ping" {
				e.sendMessage <- sendMessage{
					client: client,
					msg:    []byte("pong"),
				}
				continue
			}

			req, err := msgPkg.ParseRequest(mess)
			if err != nil {
				e.sendMessage <- sendMessage{
					client: client,
					msg:    []byte(responseMust(err, nil)),
				}
				continue
			}

			e.handleRequest(&Request{
				client:  client,
				Request: req,
			})
		}
	}
}

func (e *Epoll) Write() {
	for mess := range e.sendMessage {
		mess.client.conn.SetWriteDeadline(time.Now().Add(writeWait))

		w, err := mess.client.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			if err := e.Remove(mess.client); err != nil {
				log.Printf("Failed to remove %v\n", err)
			}
			mess.client.Close()
			continue
		}

		w.Write(mess.msg)
		if err := w.Close(); err != nil {
			if err := e.Remove(mess.client); err != nil {
				log.Printf("Failed to remove %v\n", err)
			}
			mess.client.Close()
			continue
		}
	}
}
