package main

// FeedBrokerClient to register in the broker
type FeedBrokerClient struct {
	UserID string
	Ch     chan FeedItem
}

// FeedBroker fan-outs from the notifier to all clients
type FeedBroker struct {
	Clients  map[FeedBrokerClient]struct{}
	Notifier chan FeedItem
}

func newFeedBroker() *FeedBroker {
	b := FeedBroker{
		Clients:  make(map[FeedBrokerClient]struct{}),
		Notifier: make(chan FeedItem, 1),
	}
	go b.loop()
	return &b
}

func (b *FeedBroker) loop() {
	for feedItem := range b.Notifier {
		for client := range b.Clients {
			if client.UserID == feedItem.UserID {
				client.Ch <- feedItem
			}
		}
	}
}

func (b *FeedBroker) subscribe(userID string) (chan FeedItem, func()) {
	ch := make(chan FeedItem)
	client := FeedBrokerClient{userID, ch}
	b.Clients[client] = struct{}{}
	return ch, func() {
		delete(b.Clients, client)
		close(ch)
	}
}
