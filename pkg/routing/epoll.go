package routing

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"syscall"
	"time"

	"log"

	"github.com/gorilla/websocket"
	"github.com/shinhagunn/websocket/config"
	msgPkg "github.com/shinhagunn/websocket/pkg/message"
	"github.com/twmb/franz-go/pkg/kgo"

	"golang.org/x/sys/unix"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum cap messages in channel
	maxBufferedMessages = 256
)

type Request struct {
	client *Client
	msgPkg.Request
}

type SendMessager struct {
	client *Client
	msg    []byte
}

type Event struct {
	Scope  string // global, public, private
	Stream string // channel routing key
	Type   string // event type
	Topic  string // topic routing key (stream.type)
	Body   []byte // event json body
}

func NewSendMessager(client *Client, msg []byte) SendMessager {
	return SendMessager{
		client: client,
		msg:    msg,
	}
}

// Epoll manage handler clients
type Epoll struct {
	// Epoll file descriptor
	fd int

	// The websocket connections
	Connections map[int]*Client

	// Buffered channel of outbound messages.
	send chan SendMessager

	// List of clients registered to public topics
	PublicTopics map[string]*Topic

	// List of clients registered to private topics
	PrivateTopics map[string]map[string]*Topic

	// map[prefix -> map[topic -> *Topic]]
	PrefixedTopics map[string]map[string]*Topic

	// map[prefix -> allowed roles]
	Config *config.Config

	mutex *sync.RWMutex
}

func NewEpoll(config *config.Config) (*Epoll, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	return &Epoll{
		fd:             fd,
		Connections:    make(map[int]*Client),
		send:           make(chan SendMessager, maxBufferedMessages),
		PublicTopics:   make(map[string]*Topic),
		PrivateTopics:  make(map[string]map[string]*Topic),
		PrefixedTopics: make(map[string]map[string]*Topic),
		Config:         config,
		mutex:          &sync.RWMutex{},
	}, nil
}

func (e *Epoll) Add(client *Client) error {
	fd := websocketFD(client.conn)

	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_ADD, fd, &unix.EpollEvent{Events: unix.POLLIN | unix.POLLHUP, Fd: int32(fd)})
	if err != nil {
		return err
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.Connections[fd] = client
	if len(e.Connections)%100 == 0 {
		log.Printf("Total number of connections: %v\n", len(e.Connections))
	}

	return nil
}

func (e *Epoll) Remove(client *Client) error {
	fd := websocketFD(client.conn)

	err := unix.EpollCtl(e.fd, syscall.EPOLL_CTL_DEL, fd, nil)
	if err != nil {
		return err
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	delete(e.Connections, fd)
	if len(e.Connections)%100 == 0 {
		log.Printf("Total number of connections: %v\n", len(e.Connections))
	}

	e.unsubscribeAll(client)
	client.Close()

	return nil
}

func (e *Epoll) Wait() ([]*Client, error) {
	events := make([]unix.EpollEvent, 100)
	n, err := unix.EpollWait(e.fd, events, 100)
	if err != nil {
		return nil, err
	}

	e.mutex.RLock()
	defer e.mutex.RUnlock()

	var connections []*Client
	for i := 0; i < n; i++ {
		client := e.Connections[int(events[i].Fd)]
		connections = append(connections, client)
	}

	return connections, nil
}

// ReceiveMsg handles AMQP messages
func (e *Epoll) ReceiveMsg(msg *kgo.Record) {
	keySplited := strings.Split(string(msg.Key), ".") // public.ethusdt.depth | private.UIDABC00001.balance
	scope := keySplited[0]

	e.routeMessage(&Event{
		Scope:  scope,
		Stream: keySplited[1],
		Type:   keySplited[2],
		Topic:  getTopic(scope, keySplited[1], keySplited[2]),
		Body:   msg.Value,
	})
}

func (e *Epoll) routeMessage(msg *Event) {
	log.Printf("Routing message %v\n", msg)
	e.mutex.Lock()
	defer e.mutex.Unlock()

	switch msg.Scope {
	case "public", "global":
		topic, ok := e.PublicTopics[msg.Topic]
		if ok {
			topic.broadcast(msg)
		}

		if !ok {
			log.Printf("No public registration to %s\n", msg.Topic)
			log.Printf("Public topics: %v\n", e.PublicTopics)
		}
	case "private":
		uid := msg.Stream
		uTopic, ok := e.PrivateTopics[uid]
		if ok {
			topic, ok := uTopic[msg.Topic]
			if ok {
				topic.broadcast(msg)
				break
			}
		}
		log.Printf("No private registration to %s\n", msg.Topic)
		log.Printf("Private topics: %v\n", e.PrivateTopics)
	default:
		scope, ok := e.PrefixedTopics[msg.Scope]
		if !ok {
			return
		}

		topic, ok := scope[msg.Topic]
		if !ok {
			return
		}

		topic.broadcast(msg)

		log.Printf("Broadcasted message scope %s\n", msg.Scope)
	}
}

func (e *Epoll) handleRequest(req *Request) {
	switch req.Method {
	case "subscribe":
		e.handleSubscribe(req)
	case "unsubscribe":
		e.handleUnsubscribe(req)
	default:
		e.send <- NewSendMessager(req.client, []byte(responseMust(errors.New("unsupported method"), nil)))
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
				continue
			}

			mess = bytes.TrimSpace(bytes.Replace(mess, []byte{'\n'}, []byte{' '}, -1))
			if len(mess) == 0 {
				continue
			}
			log.Printf("Received message %s\n", mess)

			if string(mess) == "ping" {
				e.send <- NewSendMessager(client, []byte("pong"))
				continue
			}

			req, err := msgPkg.ParseRequest(mess)
			if err != nil {
				e.send <- NewSendMessager(client, []byte(responseMust(err, nil)))
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
	for mess := range e.send {
		mess.client.conn.SetWriteDeadline(time.Now().Add(writeWait))

		w, err := mess.client.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			log.Println("vao day 2")
			if err := e.Remove(mess.client); err != nil {
				log.Printf("Failed to remove %v\n", err)
			}
			mess.client.Close()
			continue
		}

		w.Write(mess.msg)
		if err := w.Close(); err != nil {
			log.Println("vao day 3")
			if err := e.Remove(mess.client); err != nil {
				log.Printf("Failed to remove %v\n", err)
			}
			mess.client.Close()
			continue
		}
	}
}
