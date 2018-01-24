package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/cockroach-go/crdb"
	"github.com/go-chi/chi"
)

// Post model
type Post struct {
	ID            string    `json:"id"`
	Content       string    `json:"content"`
	SpoilerOf     *string   `json:"spoilerOf"`
	LikesCount    int       `json:"likesCount"`
	CommentsCount int       `json:"commentsCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UserID        string    `json:"-"`
	User          *User     `json:"user,omitempty"`
	Mine          bool      `json:"mine"`
	Liked         bool      `json:"liked"`
	Subscribed    bool      `json:"subscribed"`
}

// CreatePostInput request body
type CreatePostInput struct {
	Content   string  `json:"content"`
	SpoilerOf *string `json:"spoilerOf,omitempty"`
}

// TogglePostLikePayload response body
type TogglePostLikePayload struct {
	Liked      bool `json:"liked"`
	LikesCount int  `json:"likesCount"`
}

// Validate user input
func (input *CreatePostInput) Validate() map[string]string {
	// TODO: actual validation
	return nil
}

func createPost(w http.ResponseWriter, r *http.Request) {
	var input CreatePostInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if errs := input.Validate(); len(errs) != 0 {
		respondJSON(w, errs, http.StatusUnprocessableEntity)
		return
	}

	content := input.Content
	spoilerOf := input.SpoilerOf

	ctx := r.Context()
	authUser := ctx.Value(keyAuthUser).(User)

	var post Post
	var feedItem FeedItem
	if err := crdb.ExecuteTx(ctx, db, nil, func(tx *sql.Tx) error {
		if err := tx.QueryRow(`
			INSERT INTO posts (content, spoiler_of, user_id) VALUES ($1, $2, $3)
			RETURNING id, created_at
		`, content, spoilerOf, authUser.ID).Scan(&post.ID, &post.CreatedAt); err != nil {
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO subscriptions (user_id, post_id) VALUES ($1, $2)
			RETURNING NOTHING
		`, authUser.ID, post.ID); err != nil {
			return err
		}

		return tx.QueryRow(`
			INSERT INTO feed (user_id, post_id) VALUES ($1, $2)
			RETURNING id
		`, authUser.ID, post.ID).Scan(&feedItem.ID)
	}); err != nil {
		respondError(w, fmt.Errorf("could not create post: %v", err))
		return
	}

	post.Content = content
	post.SpoilerOf = spoilerOf
	post.UserID = authUser.ID
	post.User = &authUser
	post.Mine = true
	post.Subscribed = true
	feedItem.Post = post

	go feedFanout(post)
	go postMentionNotificationFanout(post)

	respondJSON(w, feedItem, http.StatusCreated)
}

// TODO: add pagination
func getPosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID, authenticated := ctx.Value(keyAuthUserID).(string)
	username := chi.URLParam(r, "username")

	query := `
		SELECT
			posts.id,
			posts.content,
			posts.spoiler_of,
			posts.likes_count,
			posts.comments_count,
			posts.created_at`
	args := []interface{}{username}
	if authenticated {
		query += `,
			posts.user_id = $2 AS mine,
			likes.user_id IS NOT NULL AS liked,
			subscriptions.user_id IS NOT NULL AS subscribed`
		args = append(args, authUserID)
	}
	query += `
		FROM posts`
	if authenticated {
		query += `
			LEFT JOIN post_likes AS likes
				ON likes.user_id = $2
				AND likes.post_id = posts.id
			LEFT JOIN subscriptions
				ON subscriptions.user_id = $2
				AND subscriptions.post_id = posts.id`
	}
	query += `
		WHERE posts.user_id = (
			SELECT id FROM users WHERE username = $1
		)
		ORDER BY posts.created_at DESC`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		respondError(w, fmt.Errorf("could not query posts: %v", err))
		return
	}
	defer rows.Close()

	posts := make([]Post, 0)
	for rows.Next() {
		var post Post
		dest := []interface{}{
			&post.ID,
			&post.Content,
			&post.SpoilerOf,
			&post.LikesCount,
			&post.CommentsCount,
			&post.CreatedAt,
		}
		if authenticated {
			dest = append(dest,
				&post.Mine,
				&post.Liked,
				&post.Subscribed,
			)
		}

		if err = rows.Scan(dest...); err != nil {
			respondError(w, fmt.Errorf("could not scan post: %v", err))
			return
		}

		posts = append(posts, post)
	}
	if err = rows.Err(); err != nil {
		respondError(w, fmt.Errorf("could not iterate over posts: %v", err))
		return
	}

	respondJSON(w, posts, http.StatusOK)
}

func getPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID, authenticated := ctx.Value(keyAuthUserID).(string)
	postID := chi.URLParam(r, "post_id")

	query := `
		SELECT
			posts.content,
			posts.spoiler_of,
			posts.likes_count,
			posts.comments_count,
			posts.created_at,
			users.username,
			users.avatar_url`
	args := []interface{}{postID}
	if authenticated {
		query += `,
			posts.user_id = $2 AS mine,
			EXISTS (
				SELECT 1 FROM post_likes
				WHERE user_id = $2 AND post_id = $1
			) AS liked,
			EXISTS (
				SELECT 1 FROM subscriptions
				WHERE user_id = $2 AND post_id = $1
			) AS subscribed`
		args = append(args, authUserID)
	}
	query += `
		FROM posts
		INNER JOIN users ON posts.user_id = users.id
		WHERE posts.id = $1`
	var user User
	var post Post
	dest := []interface{}{
		&post.Content,
		&post.SpoilerOf,
		&post.LikesCount,
		&post.CommentsCount,
		&post.CreatedAt,
		&user.Username,
		&user.AvatarURL,
	}
	if authenticated {
		dest = append(dest,
			&post.Mine,
			&post.Liked,
			&post.Subscribed,
		)
	}

	if err := db.QueryRowContext(ctx, query, args...).Scan(dest...); err == sql.ErrNoRows {
		http.Error(w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	} else if err != nil {
		respondError(w, fmt.Errorf("could not get post: %v", err))
		return
	}

	post.ID = postID
	post.User = &user

	respondJSON(w, post, http.StatusOK)
}

func togglePostLike(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID := ctx.Value(keyAuthUserID).(string)
	postID := chi.URLParam(r, "post_id")

	var liked bool
	var likesCount int
	if err := crdb.ExecuteTx(ctx, db, nil, func(tx *sql.Tx) error {

		if err := tx.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM post_likes
			WHERE user_id = $1 AND post_id = $2
		)`, authUserID, postID).Scan(&liked); err != nil {
			return err
		}

		if liked {
			if _, err := tx.Exec(`
				DELETE FROM post_likes
				WHERE user_id = $1 AND post_id = $2
				RETURNING NOTHING
			`, authUserID, postID); err != nil {
				return err
			}

			return tx.QueryRow(`
				UPDATE posts SET likes_count = likes_count - 1
				WHERE id = $1
				RETURNING likes_count
			`, postID).Scan(&likesCount)
		}

		if _, err := tx.Exec(`
			INSERT INTO post_likes (user_id, post_id) VALUES ($1, $2)
			RETURNING NOTHING
		`, authUserID, postID); err != nil {
			return err
		}

		return tx.QueryRow(`
			UPDATE posts SET likes_count = likes_count + 1
			WHERE id = $1
			RETURNING likes_count
		`, postID).Scan(&likesCount)
	}); err != nil {
		respondError(w, fmt.Errorf("could not toggle post like: %v", err))
		return
	}

	liked = !liked

	respondJSON(w, TogglePostLikePayload{liked, likesCount}, http.StatusOK)
}

func toggleSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID := ctx.Value(keyAuthUserID).(string)
	postID := chi.URLParam(r, "post_id")

	var subscribed bool
	if err := crdb.ExecuteTx(ctx, db, nil, func(tx *sql.Tx) error {
		if err := tx.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM subscriptions
			WHERE user_id = $1 AND post_id = $2
		)`, authUserID, postID).Scan(&subscribed); err != nil {
			return err
		}

		if subscribed {
			_, err := tx.Exec(`
				DELETE FROM subscriptions
				WHERE user_id = $1 AND post_id = $2
				RETURNING NOTHING
			`, authUserID, postID)
			return err
		}

		_, err := tx.Exec(`
			INSERT INTO subscriptions (user_id, post_id) VALUES ($1, $2)
			RETURNING NOTHING
		`, authUserID, postID)
		return err
	}); err != nil {
		respondError(w, fmt.Errorf("could not toggle subscription: %v", err))
		return
	}

	subscribed = !subscribed

	respondJSON(w, subscribed, http.StatusOK)
}
