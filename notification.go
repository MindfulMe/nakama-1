package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach-go/crdb"
	"github.com/gernest/mention"
	"github.com/lib/pq"
)

// Notification model
type Notification struct {
	ID            string    `json:"id"`
	UserID        string    `json:"-"`
	ActorID       string    `json:"-"`
	Verb          string    `json:"verb"`
	ObjectID      *string   `json:"objectId,omitempty"`
	TargetID      *string   `json:"targetId,omitempty"`
	IssuedAt      time.Time `json:"issuedAt"`
	Read          bool      `json:"read"`
	ActorUsername string    `json:"actorUsername"`
}

func getNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID := ctx.Value(keyAuthUserID).(string)

	if a := r.Header.Get("Accept"); strings.Contains(a, "text/event-stream") {
		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		h := w.Header()
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("Content-Type", "text/event-stream")

		ch, unsubscribe := notificationsBroker.subscribe(authUserID)
		defer unsubscribe()

		for {
			select {
			case <-w.(http.CloseNotifier).CloseNotify():
				return
			case <-time.After(time.Second * 15):
				fmt.Fprint(w, "ping: \n\n")
				f.Flush()
			case notification := <-ch:
				if b, err := json.Marshal(notification); err != nil {
					fmt.Fprintf(w, "error: %v\n\n", err)
				} else {
					fmt.Fprintf(w, "data: %s\n\n", b)
				}
				f.Flush()
			}
		}
	}

	rows, err := db.QueryContext(ctx, `
		SELECT
			notifications.id,
			actors.username,
			notifications.verb,
			notifications.object_id,
			notifications.target_id,
			notifications.issued_at,
			notifications.issued_at <= users.notifications_seen_at AS read
		FROM notifications
		INNER JOIN users AS actors ON notifications.actor_id = actors.id
		INNER JOIN users ON notifications.user_id = users.id
		WHERE notifications.user_id = $1
		ORDER BY notifications.issued_at DESC
	`, authUserID)
	if err != nil {
		respondError(w, fmt.Errorf("could not query notifications: %v", err))
		return
	}
	defer rows.Close()

	notifications := make([]Notification, 0)
	for rows.Next() {
		var notification Notification
		if err = rows.Scan(
			&notification.ID,
			&notification.ActorUsername,
			&notification.Verb,
			&notification.ObjectID,
			&notification.TargetID,
			&notification.IssuedAt,
			&notification.Read,
		); err != nil {
			respondError(w, fmt.Errorf("could not scan notification: %v", err))
			return
		}

		notifications = append(notifications, notification)
	}

	if err = rows.Err(); err != nil {
		respondError(w, fmt.Errorf("could not iterate over notifications: %v", err))
		return
	}

	go updateNotificationsSeenAt(authUserID)

	respondJSON(w, notifications, http.StatusOK)
}

func updateNotificationsSeenAt(userID string) {
	if _, err := db.Exec(`
		UPDATE users SET
			notifications_seen_at = now()
		WHERE id = $1
	`, userID); err != nil {
		log.Printf("could not update notifications seen at: %v\n", err)
	}
}

func checkUnreadNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID := ctx.Value(keyAuthUserID).(string)

	var unread bool
	if err := db.QueryRowContext(ctx, `
		SELECT notifications.issued_at > users.notifications_seen_at AS unread
		FROM notifications
		INNER JOIN users ON notifications.user_id = users.id
		WHERE notifications.user_id = $1
		ORDER BY notifications.issued_at DESC
		LIMIT 1
	`, authUserID).Scan(&unread); err != nil && err != sql.ErrNoRows {
		respondError(w, fmt.Errorf("could not check unread notifications: %v", err))
		return
	}

	respondJSON(w, unread, http.StatusOK)
}

func createFollowNotification(follower User, followingID string) {
	var exists bool
	var notification Notification
	if err := crdb.ExecuteTx(context.Background(), db, nil, func(tx *sql.Tx) error {
		if err := tx.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM notifications
			WHERE user_id = $1
				AND actor_id = $2
				AND verb = 'follow'
		)`, followingID, follower.ID).Scan(&exists); err != nil {
			return err
		}

		if exists {
			return nil
		}

		return tx.QueryRow(`
			INSERT INTO notifications (user_id, actor_id, verb) VALUES ($1, $2, 'follow')
			RETURNING id, issued_at
		`, followingID, follower.ID).Scan(&notification.ID, &notification.IssuedAt)
	}); err != nil {
		log.Printf("could not create follow notification: %v\n", err)
		return
	}

	notification.UserID = followingID
	notification.ActorID = follower.ID
	notification.Verb = "follow"
	notification.ActorUsername = follower.Username
	created := !exists

	if created {
		notificationsBroker.Notifier <- notification
	}
}

func commentNotificationFanout(comment Comment) {
	rows, err := db.Query(`
		INSERT INTO notifications (user_id, actor_id, verb, object_id, target_id)
		SELECT user_id, $1, 'comment', $2, $3
		FROM subscriptions
		WHERE user_id != $1 AND post_id = $3
		RETURNING id, user_id, issued_at
	`, comment.UserID, comment.ID, comment.PostID)
	if err != nil {
		log.Printf("could not query comment notification fanout: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var notification Notification
		if err = rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.IssuedAt,
		); err != nil {
			log.Printf("could not scan comment notification fanout: %v\n", err)
			return
		}

		notification.ActorID = comment.UserID
		notification.Verb = "comment"
		notification.ObjectID = &comment.ID
		notification.TargetID = &comment.PostID
		notification.ActorUsername = comment.User.Username

		notificationsBroker.Notifier <- notification
	}

	if err = rows.Err(); err != nil {
		log.Printf("could not iterate over comment notification fanout: %v\n", err)
	}
}

func collectMentions(content string) []string {
	return mention.GetTags('@', strings.NewReader(content), ',', '.', '!', '?', '"', ')')
}

func postMentionNotificationFanout(post Post) {
	usernames := collectMentions(post.Content)
	if len(usernames) == 0 {
		return
	}

	rows, err := db.Query(`
		INSERT INTO notifications (user_id, actor_id, verb, object_id)
		SELECT id, $1, 'post_mention', $2
		FROM users
		WHERE id != $1
			AND username = ANY($3)
		RETURNING id, user_id, issued_at
	`, post.UserID, post.ID, pq.Array(usernames))
	if err != nil {
		log.Printf("could not query post mention notification fanout: %v\n", err)
		return
	}
	defer rows.Err()

	for rows.Next() {
		var notification Notification
		if err = rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.IssuedAt,
		); err != nil {
			log.Printf("could not scan post mention notification fanout: %v\n", err)
			return
		}

		notification.ActorID = post.UserID
		notification.Verb = "post_mention"
		notification.ObjectID = &post.ID
		notification.ActorUsername = post.User.Username

		notificationsBroker.Notifier <- notification
	}

	if err = rows.Err(); err != nil {
		log.Printf("could not iterate over post mention notification fanout: %v\n", err)
	}
}

func commentMentionNotificationFanout(comment Comment) {
	usernames := collectMentions(comment.Content)
	if len(usernames) == 0 {
		return
	}

	rows, err := db.Query(`
		INSERT INTO notifications (user_id, actor_id, verb, object_id, target_id)
		SELECT id, $1, 'comment_mention', $2, $3
		FROM users
		WHERE id != $1
			AND username = ANY($4)
		RETURNING id, user_id, issued_at
	`, comment.UserID, comment.ID, comment.PostID, pq.Array(usernames))
	if err != nil {
		log.Printf("could not query comment mention notification fanout: %v\n", err)
		return
	}
	defer rows.Err()

	for rows.Next() {
		var notification Notification
		if err = rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.IssuedAt,
		); err != nil {
			log.Printf("could not scan comment mention notification fanout: %v\n", err)
			return
		}

		notification.ActorID = comment.UserID
		notification.Verb = "comment_mention"
		notification.ObjectID = &comment.ID
		notification.TargetID = &comment.PostID
		notification.ActorUsername = comment.User.Username

		notificationsBroker.Notifier <- notification
	}

	if err = rows.Err(); err != nil {
		log.Printf("could not iterate over comment mention notification fanout: %v\n", err)
	}
}
