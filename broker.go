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

// CommentsBrokerClient to register in the broker
type CommentsBrokerClient struct {
	UserID string
	PostID string
	Ch     chan Comment
}

// CommentsBroker fan-outs from the notifier to all clients
type CommentsBroker struct {
	Clients  map[CommentsBrokerClient]struct{}
	Notifier chan Comment
}

// NotificationsBrokerClient to register in the broker
type NotificationsBrokerClient struct {
	UserID string
	Ch     chan Notification
}

// NotificationsBroker fan-outs from the notifier to all clients
type NotificationsBroker struct {
	Clients  map[NotificationsBrokerClient]struct{}
	Notifier chan Notification
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

func newCommentsBroker() *CommentsBroker {
	b := CommentsBroker{
		Clients:  make(map[CommentsBrokerClient]struct{}),
		Notifier: make(chan Comment, 1),
	}
	return &b
}

func (b *CommentsBroker) loop() {
	for comment := range b.Notifier {
		for client := range b.Clients {
			// We don't notify the owner
			if client.UserID != comment.UserID && client.PostID == comment.PostID {
				client.Ch <- comment
			}
		}
	}
}

func (b *CommentsBroker) subscribe(userID, postID string) (chan Comment, func()) {
	ch := make(chan Comment)
	client := CommentsBrokerClient{userID, postID, ch}
	b.Clients[client] = struct{}{}
	return ch, func() {
		delete(b.Clients, client)
		close(ch)
	}
}

func newNotificationsBroker() *NotificationsBroker {
	b := NotificationsBroker{
		Clients:  make(map[NotificationsBrokerClient]struct{}),
		Notifier: make(chan Notification, 1),
	}
	go b.loop()
	return &b
}

func (b *NotificationsBroker) loop() {
	for notification := range b.Notifier {
		for client := range b.Clients {
			if client.UserID == notification.UserID {
				client.Ch <- notification
			}
		}
	}
}

func (b *NotificationsBroker) subscribe(userID string) (chan Notification, func()) {
	ch := make(chan Notification)
	client := NotificationsBrokerClient{userID, ch}
	b.Clients[client] = struct{}{}
	return ch, func() {
		delete(b.Clients, client)
		close(ch)
	}
}
