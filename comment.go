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

// CreateCommentInput request body
type CreateCommentInput struct {
	Content string `json:"content"`
}

// Comment model
type Comment struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	LikesCount int       `json:"likesCount"`
	CreatedAt  time.Time `json:"createdAt"`
	UserID     string    `json:"-"`
	PostID     string    `json:"-"`
	User       User      `json:"user"`
	Mine       bool      `json:"mine"`
	Liked      bool      `json:"liked"`
}

// ToggleCommentLikePayload response body
type ToggleCommentLikePayload struct {
	Liked      bool `json:"liked"`
	LikesCount int  `json:"likesCount"`
}

func createComment(w http.ResponseWriter, r *http.Request) {
	// Decode JSON body
	var input CreateCommentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	content := input.Content
	// TODO: validate input
	// Store comment and increment post's comments_count
	ctx := r.Context()
	authUser := ctx.Value(keyAuthUser).(User)
	postID := chi.URLParam(r, "post_id")
	var comment Comment
	if err := crdb.ExecuteTx(ctx, db, nil, func(tx *sql.Tx) error {
		if err := tx.QueryRow(`
			INSERT INTO comments (content, user_id, post_id) VALUES ($1, $2, $3)
			RETURNING id, created_at
		`, content, authUser.ID, postID).Scan(&comment.ID, &comment.CreatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(`
			UPDATE posts SET comments_count = comments_count + 1
			WHERE id = $1
			RETURNING NOTHING
		`, postID)
		return err
	}); err != nil {
		respondError(w, fmt.Errorf("could not create comment: %v", err))
		return
	}
	comment.Content = content
	comment.UserID = authUser.ID
	comment.PostID = postID
	comment.User = authUser
	comment.Mine = true
	// TODO: broadcast and notify
	// Respond with created comment
	respondJSON(w, comment, http.StatusCreated)
}

func getComments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID, authenticated := ctx.Value(keyAuthUserID).(string)
	postID := chi.URLParam(r, "post_id")
	// Build a query based on auth state
	query := `
		SELECT
			comments.id,
			comments.content,
			comments.likes_count,
			comments.created_at,
			users.username,
			users.avatar_url`
	args := []interface{}{postID}
	if authenticated {
		query += `,
			comments.user_id = $2 AS mine,
			likes.user_id IS NOT NULL AS liked`
		args = append(args, authUserID)
	}
	query += `
		FROM comments
		INNER JOIN users ON comments.user_id = users.id`
	if authenticated {
		query += `
			LEFT JOIN comment_likes AS likes
			ON likes.user_id = $2 AND likes.comment_id = comments.id`
	}
	query += `
		WHERE comments.post_id = $1
		ORDER BY comments.created_at DESC`
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		respondError(w, fmt.Errorf("could not query comments: %v", err))
		return
	}
	defer rows.Close()
	// Scan each row
	comments := make([]Comment, 0)
	for rows.Next() {
		var user User
		var comment Comment
		dest := []interface{}{
			&comment.ID,
			&comment.Content,
			&comment.LikesCount,
			&comment.CreatedAt,
			&user.Username,
			&user.AvatarURL,
		}
		if authenticated {
			dest = append(dest,
				&comment.Mine,
				&comment.Liked,
			)
		}
		if err = rows.Scan(dest...); err != nil {
			respondError(w, fmt.Errorf("could not scan comment: %v", err))
			return
		}
		comment.User = user
		comments = append(comments, comment)
	}
	if err = rows.Err(); err != nil {
		respondError(w, fmt.Errorf("could not iterate over comments: %v", err))
		return
	}
	// Respond with array of comments
	respondJSON(w, comments, http.StatusOK)
}

func toggleCommentLike(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID := ctx.Value(keyAuthUserID).(string)
	commentID := chi.URLParam(r, "comment_id")
	var liked bool
	var likesCount int
	if err := crdb.ExecuteTx(ctx, db, nil, func(tx *sql.Tx) error {
		// Check if liked
		if err := tx.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM comment_likes
			WHERE user_id = $1 AND comment_id = $2
		)`, authUserID, commentID).Scan(&liked); err != nil {
			return err
		}
		// If so, delete like and decrement comment's likes_count
		if liked {
			if _, err := tx.Exec(`
				DELETE FROM comment_likes
				WHERE user_id = $1 AND comment_id = $2
				RETURNING NOTHING
			`, authUserID, commentID); err != nil {
				return err
			}
			return tx.QueryRow(`
				UPDATE comments SET likes_count = likes_count - 1
				WHERE id = $1
				RETURNING likes_count
			`, commentID).Scan(&likesCount)
		}
		// Else, insert like and increment comment's likes_count
		if _, err := tx.Exec(`
			INSERT INTO comment_likes (user_id, comment_id) VALUES ($1, $2)
			RETURNING NOTHING
		`, authUserID, commentID); err != nil {
			return err
		}
		return tx.QueryRow(`
			UPDATE comments SET likes_count = likes_count + 1
			WHERE id = $1
			RETURNING likes_count
		`, commentID).Scan(&likesCount)
	}); err != nil {
		respondError(w, fmt.Errorf("could not toggle comment like: %v", err))
		return
	}
	liked = !liked
	// Respond with results
	respondJSON(w, ToggleCommentLikePayload{liked, likesCount}, http.StatusOK)
}
