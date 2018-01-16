package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// FeedItem model
type FeedItem struct {
	ID     string `json:"id"`
	UserID string `json:"-"`
	PostID string `json:"-"`
	Post   Post   `json:"post"`
}

// TODO: add pagination
func getFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID := ctx.Value(keyAuthUserID).(string)

	if a := r.Header.Get("Accept"); strings.HasPrefix(a, "text/event-stream") {
		f, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		h := w.Header()
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("Content-Type", "text/event-stream")

		ch, unsubscribe := feedBroker.subscribe(authUserID)
		defer unsubscribe()

		for {
			select {
			case <-w.(http.CloseNotifier).CloseNotify():
				return
			case <-time.After(time.Second * 15):
				fmt.Fprint(w, "ping: \n\n")
				f.Flush()
			case feedItem := <-ch:
				if b, err := json.Marshal(feedItem); err != nil {
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
			feed.id,
			posts.id,
			posts.content,
			posts.spoiler_of,
			posts.likes_count,
			posts.comments_count,
			posts.created_at,
			users.username,
			users.avatar_url,
			posts.user_id = $1 AS mine,
			likes.user_id IS NOT NULL AS liked,
			subscriptions.user_id IS NOT NULL AS subscribed
		FROM feed
		INNER JOIN posts ON feed.post_id = posts.id
		INNER JOIN users ON posts.user_id = users.id
		LEFT JOIN post_likes AS likes
			ON likes.user_id = $1
			AND likes.post_id = posts.id
		LEFT JOIN subscriptions
			ON subscriptions.user_id = $1
			AND subscriptions.post_id = posts.id
		WHERE feed.user_id = $1
		ORDER BY posts.created_at DESC
	`, authUserID)
	if err != nil {
		respondError(w, fmt.Errorf("could not query feed: %v", err))
		return
	}
	defer rows.Close()

	feed := make([]FeedItem, 0)
	for rows.Next() {
		var user User
		var post Post
		var feedItem FeedItem
		if err = rows.Scan(
			&feedItem.ID,
			&post.ID,
			&post.Content,
			&post.SpoilerOf,
			&post.LikesCount,
			&post.CommentsCount,
			&post.CreatedAt,
			&user.Username,
			&user.AvatarURL,
			&post.Mine,
			&post.Liked,
			&post.Subscribed,
		); err != nil {
			respondError(w, fmt.Errorf("could not scan feed item: %v", err))
			return
		}

		post.User = &user
		feedItem.Post = post
		feed = append(feed, feedItem)
	}
	if err = rows.Err(); err != nil {
		respondError(w, fmt.Errorf("could not iterate over feed: %v", err))
		return
	}

	respondJSON(w, feed, http.StatusOK)
}

func feedFanout(post Post) {
	post.Mine = false
	post.Subscribed = false

	rows, err := db.Query(`
		INSERT INTO feed (user_id, post_id)
		SELECT follower_id, $1 FROM follows WHERE following_id = $2
		RETURNING id, user_id
	`, post.ID, post.UserID)
	if err != nil {
		log.Printf("could not query feed fanout: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var feedItem FeedItem
		if err = rows.Scan(&feedItem.ID, &feedItem.UserID); err != nil {
			log.Printf("could not scan feed fanout: %v\n", err)
			return
		}
		feedItem.Post = post
		feedBroker.Notifier <- feedItem
	}
	if err = rows.Err(); err != nil {
		log.Printf("could not iterate over feed fanout: %v\n", err)
	}
}
