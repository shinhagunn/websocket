package routing

import (
	"log"

	"github.com/shinhagunn/websocket/pkg/message"
)

type Topic struct {
	clients map[*Client]struct{}
}

func NewTopic() *Topic {
	return &Topic{
		clients: make(map[*Client]struct{}),
	}
}

func eventMust(method string, data interface{}) []byte {
	ev, err := message.PackOutgoingEvent(method, data)
	if err != nil {
		log.Println(err.Error())
	}

	return ev
}

func contains(list []string, el string) bool {
	for _, l := range list {
		if l == el {
			return true
		}
	}
	return false
}

func (t *Topic) len() int {
	return len(t.clients)
}

// func (t *Topic) broadcast(message *Event) {
// 	var bodyMsg interface{}

// 	if err := json.Unmarshal(message.Body, &bodyMsg); err != nil {
// 		log.Errorf("Fail to JSON marshal: %s", err.Error())
// 		return
// 	}

// 	body, err := json.Marshal(map[string]interface{}{
// 		message.Topic: bodyMsg,
// 	})

// 	if err != nil {
// 		log.Errorf("Fail to JSON marshal: %s", err.Error())
// 		return
// 	}

// 	for client := range t.clients {
// 		client.Send(string(body))
// 	}
// }

// func (t *Topic) broadcastRaw(msg []byte) {
// 	for client := range t.clients {
// 		client.Send(string(msg))
// 	}
// }

func (t *Topic) subscribe(c *Client) bool {
	if _, ok := t.clients[c]; ok {
		return false
	}
	t.clients[c] = struct{}{}

	return true
}

func (t *Topic) unsubscribe(c *Client) bool {
	_, ok := t.clients[c]
	delete(t.clients, c)

	return ok
}
