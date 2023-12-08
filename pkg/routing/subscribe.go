package routing

import "log"

func (e *Epoll) handleSubscribe(req *Request) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for _, t := range req.Streams {
		switch {
		case isPrivateStream(t):
			e.subscribePrivate(t, req)
		case isPrefixedStream(t):
			e.subscribePrefixed(t, req)
		default:
			e.subscribePublic(t, req)
		}
	}

	e.send <- NewSendMessager(req.client, []byte(responseMust(nil, map[string]interface{}{
		"message": "subscribed",
		"streams": req.client.GetSubscriptions(),
	})))
}

func (e *Epoll) subscribePublic(t string, req *Request) {
	topic, ok := e.PublicTopics[t]
	if !ok {
		topic = NewTopic(e.send)
		e.PublicTopics[t] = topic
	}

	if topic.subscribe(req.client) {
		req.client.SubscribePublic(t)
	}
}

func (e *Epoll) premittedRBAC(prefix string, auth Auth) bool {
	rbac := e.Config.Rango.RbacAdmin
	if prefix == "system" {
		rbac = e.Config.Rango.RbacSystem
	}

	for _, role := range rbac {
		if role == auth.Role {
			return true
		}
	}

	return false
}

func (e *Epoll) subscribePrefixed(prefixed string, req *Request) {
	prefix, t := splitPrefixedTopic(prefixed)

	if !e.premittedRBAC(prefix, req.client.GetAuth()) {
		e.send <- NewSendMessager(req.client, []byte(responseMust(nil, map[string]interface{}{
			"message": "cannot subscribe to " + prefixed,
		})))

		return
	}

	topics, ok := e.PrefixedTopics[prefix]
	if !ok {
		topics := make(map[string]*Topic, 0)
		e.PrefixedTopics[prefix] = topics
	}

	topic, ok := topics[t]
	if !ok {
		topic = NewTopic(e.send)
		e.PrefixedTopics[prefix][t] = topic
	}

	if topic.subscribe(req.client) {
		req.client.SubscribePublic(prefixed)
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
		topic = NewTopic(e.send)
		uTopics[t] = topic
	}

	if topic.subscribe(req.client) {
		req.client.SubscribePrivate(t)
	}
}
