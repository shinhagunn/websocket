package routing

func (e *Epoll) handleUnsubscribe(req *Request) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for _, t := range req.Streams {
		switch {
		case isPrivateStream(t):
			e.unsubscribePrivate(t, req)
		case isPrefixedStream(t):
			e.unsubscribePrefixed(t, req)
		default:
			e.unsubscribePublic(t, req)
		}
	}

	e.send <- NewSendMessager(req.client, []byte(responseMust(nil, map[string]interface{}{
		"message": "unsubscribed",
		"streams": req.client.GetSubscriptions(),
	})))
}

func (e *Epoll) unsubscribeAll(client *Client) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for t, topic := range e.PublicTopics {
		topic.unsubscribe(client)
		if topic.len() == 0 {
			delete(e.PublicTopics, t)
		}
	}

	for k, scope := range e.PrefixedTopics {
		for t, topic := range scope {
			topic.unsubscribe(client)
			if topic.len() == 0 {
				delete(scope, t)
			}
		}

		if len(scope) == 0 {
			delete(e.PrefixedTopics, k)
		}
	}

	uid := client.GetAuth().UID
	topics, ok := e.PrivateTopics[uid]
	if !ok {
		return
	}

	for t, topic := range topics {
		topic.unsubscribe(client)
		if topic.len() == 0 {
			delete(topics, t)
		}
	}

	if len(topics) == 0 {
		delete(e.PrivateTopics, uid)
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

func (e *Epoll) unsubscribePrefixed(prefixed string, req *Request) {
	scope, t := splitPrefixedTopic(prefixed)
	topics, ok := e.PrefixedTopics[scope]
	if !ok {
		return
	}

	topic, ok := topics[t]
	if ok {
		if topic.unsubscribe(req.client) {
			req.client.UnsubscribePublic(t)
		}

		if topic.len() == 0 {
			delete(topics, t)
			e.PrefixedTopics[scope] = topics
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
