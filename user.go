package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach-go/crdb"
	"github.com/go-chi/chi"
	"github.com/lib/pq"
)

// User model
type User struct {
	ID        string  `json:"-"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarUrl"`
}

// Profile model
type Profile struct {
	Email           string    `json:"email,omitempty"`
	Username        string    `json:"username"`
	AvatarURL       *string   `json:"avatarUrl"`
	FollowersCount  int       `json:"followersCount"`
	FollowingCount  int       `json:"followingCount"`
	CreatedAt       time.Time `json:"createdAt"`
	Me              bool      `json:"me"`
	FollowerOfMine  bool      `json:"followerOfMine"`
	FollowingOfMine bool      `json:"followingOfMine"`
}

// CreateUserInput request body
type CreateUserInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
}

// ToggleFollowPayload response body
type ToggleFollowPayload struct {
	FollowingOfMine bool `json:"followingOfMine"`
	FollowersCount  int  `json:"followersCount"`
}

// Validate user input
func (input *CreateUserInput) Validate() map[string]string {
	// TODO: actual validation
	return nil
}

var errFollowingMyself = errors.New("Try following someone else")

func createUser(w http.ResponseWriter, r *http.Request) {
	var input CreateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if errs := input.Validate(); len(errs) != 0 {
		respondJSON(w, errs, http.StatusUnprocessableEntity)
		return
	}

	email := input.Email
	username := input.Username

	var user Profile
	err := db.QueryRowContext(r.Context(), `
		INSERT INTO users (email, username) VALUES ($1, $2)
		RETURNING created_at
	`, email, username).Scan(&user.CreatedAt)
	if errPq, ok := err.(*pq.Error); ok && errPq.Code.Name() == "unique_violation" {
		if strings.Contains(errPq.Error(), "users_email_key") {
			respondJSON(w, map[string]string{
				"email": "Email taken",
			}, http.StatusUnprocessableEntity)
			return
		}
		respondJSON(w, map[string]string{
			"username": "Username taken",
		}, http.StatusUnprocessableEntity)
		return
	} else if err != nil {
		respondError(w, fmt.Errorf("could not create user: %v", err))
		return
	}

	user.Email = email
	user.Username = username
	user.Me = true

	respondJSON(w, user, http.StatusCreated)
}

// TODO: add pagination
func getUsers(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	users, err := getUsersWhere(r.Context(),
		"users.username ILIKE '%' || $1 || '%'", username)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, users, http.StatusOK)
}

func getUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUserID, authenticated := ctx.Value(keyAuthUserID).(string)
	username := chi.URLParam(r, "username")

	query := `
		SELECT
			id,
			email,
			avatar_url,
			followers_count,
			following_count,
			created_at`
	args := []interface{}{username}
	if authenticated {
		query += `,
			EXISTS (
				SELECT 1 FROM follows
				WHERE follower_id = (SELECT id FROM users WHERE username = $1)
					AND following_id = $2
			) AS follower_of_mine,
			EXISTS (
				SELECT 1 FROM follows
				WHERE follower_id = $2
					AND following_id = (SELECT id FROM users WHERE username = $1)
			) AS following_of_mine`
		args = append(args, authUserID)
	}
	query += `
		FROM users
		WHERE username = $1`
	var userID string
	var user Profile
	dest := []interface{}{
		&userID,
		&user.Email,
		&user.AvatarURL,
		&user.FollowersCount,
		&user.FollowingCount,
		&user.CreatedAt,
	}
	if authenticated {
		dest = append(dest,
			&user.FollowerOfMine,
			&user.FollowingOfMine,
		)
	}

	if err := db.QueryRowContext(ctx, query, args...).Scan(dest...); err == sql.ErrNoRows {
		http.Error(w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	} else if err != nil {
		respondError(w, fmt.Errorf("could not get user: %v", err))
		return
	}

	if !authenticated || authUserID != userID {
		user.Email = ""
	}
	user.Username = username
	user.Me = authenticated && userID == authUserID

	respondJSON(w, user, http.StatusOK)
}

func uploadAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	ct := http.DetectContentType(b)
	if ct != "image/jpg" && ct != "image/jpeg" && ct != "image/png" {
		log.Printf("%s is not a valid image\n", ct)
		http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
		return
	}

	extensions, err := mime.ExtensionsByType(ct)
	if err != nil {
		respondError(w, fmt.Errorf("could not get avatar extension: %v", err))
		return
	}
	if extensions == nil || len(extensions) == 0 {
		respondError(w, errors.New("could not get avatar extension"))
		return
	}
	ext := extensions[0]

	ctx := r.Context()
	authUser := ctx.Value(keyAuthUser).(User)

	if err := ioutil.WriteFile(filepath.Join("avatars", authUser.ID+ext), b, os.ModePerm); err != nil {
		respondError(w, fmt.Errorf("could not write avatar to disc: %v", err))
	}

	// TODO: correctly build avatar URL
	avatarURL := "http://localhost/avatars/" + authUser.ID + ext

	if _, err := db.ExecContext(ctx, `
		UPDATE users SET avatar_url = $1
		WHERE id = $2
	`, avatarURL, authUser.ID); err != nil {
		respondError(w, fmt.Errorf("could not update avatar_url: %v", err))
		return
	}

	io.WriteString(w, avatarURL)
}

func toggleFollow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUser := ctx.Value(keyAuthUser).(User)
	username := chi.URLParam(r, "username")

	var userID string
	var followingOfMine bool
	var followersCount int
	if err := crdb.ExecuteTx(ctx, db, nil, func(tx *sql.Tx) error {
		if err := tx.QueryRow("SELECT id FROM users WHERE username = $1", username).
			Scan(&userID); err != nil {
			return err
		}

		if authUser.ID == userID {
			return errFollowingMyself
		}

		if err := tx.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM follows
			WHERE follower_id = $1
				AND following_id = $2
		)`, authUser.ID, userID).Scan(&followingOfMine); err != nil {
			return err
		}

		if followingOfMine {
			if _, err := tx.Exec(`
				DELETE FROM follows
				WHERE follower_id = $1
					AND following_id = $2
				RETURNING NOTHING
			`, authUser.ID, userID); err != nil {
				return err
			}

			if _, err := tx.Exec(`
				UPDATE users SET following_count = following_count - 1
				WHERE id = $1
				RETURNING NOTHING
			`, authUser.ID); err != nil {
				return err
			}

			return tx.QueryRow(`
				UPDATE users SET followers_count = followers_count - 1
				WHERE id = $1
				RETURNING followers_count
			`, userID).Scan(&followersCount)
		}

		if _, err := tx.Exec(`
			INSERT INTO follows (follower_id, following_id)
			VALUES ($1, $2)
			RETURNING NOTHING
		`, authUser.ID, userID); err != nil {
			return err
		}

		if _, err := tx.Exec(`
			UPDATE users SET following_count = following_count + 1
			WHERE id = $1
			RETURNING NOTHING
		`, authUser.ID); err != nil {
			return err
		}

		return tx.QueryRow(`
			UPDATE users SET followers_count = followers_count + 1
			WHERE id = $1
			RETURNING followers_count
		`, userID).Scan(&followersCount)
	}); err == errFollowingMyself {
		http.Error(w,
			http.StatusText(http.StatusForbidden),
			http.StatusForbidden)
		return
	} else if err != nil {
		respondError(w, fmt.Errorf("could not toggle follow: %v", err))
		return
	}

	followingOfMine = !followingOfMine

	if followingOfMine {
		go createFollowNotification(authUser, userID)
	}

	respondJSON(w, ToggleFollowPayload{followingOfMine, followersCount}, http.StatusOK)
}

func getFollowers(w http.ResponseWriter, r *http.Request) {
	users, err := getUsersWhere(r.Context(), `users.id IN (
		SELECT follower_id
		FROM follows
		WHERE following_id = (
			SELECT id FROM users WHERE username = $1
		)
	)`, chi.URLParam(r, "username"))
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, users, http.StatusOK)
}

func getFollowing(w http.ResponseWriter, r *http.Request) {
	users, err := getUsersWhere(r.Context(), `users.id IN (
		SELECT following_id
		FROM follows
		WHERE follower_id = (
			SELECT id FROM users WHERE username = $1
		)
	)`, chi.URLParam(r, "username"))
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, users, http.StatusOK)
}

func getUsersWhere(ctx context.Context, where, username string) ([]Profile, error) {
	authUserID, authenticated := ctx.Value(keyAuthUserID).(string)

	query := `
		SELECT
			users.username,
			users.avatar_url,
			users.followers_count,
			users.following_count,
			users.created_at`
	args := []interface{}{username}
	if authenticated {
		query += `,
			following.following_id IS NOT NULL AS follower_of_mine,
			followers.follower_id IS NOT NULL AS following_of_mine`
		args = append(args, authUserID)
	}
	query += `
		FROM users`
	if authenticated {
		query += `
			LEFT JOIN follows AS followers
				ON followers.follower_id = $2
				AND followers.following_id = users.id
			LEFT JOIN follows AS following
				ON following.follower_id = users.id
				AND following.following_id = $2
			WHERE users.id != $2 AND`
	} else {
		query += `
			WHERE`
	}

	query += fmt.Sprintf(" %s\nORDER BY users.username", where)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query users: %v", err)
	}
	defer rows.Close()

	users := make([]Profile, 0)
	for rows.Next() {
		var user Profile
		dest := []interface{}{
			&user.Username,
			&user.AvatarURL,
			&user.FollowersCount,
			&user.FollowingCount,
			&user.CreatedAt,
		}
		if authenticated {
			dest = append(dest,
				&user.FollowerOfMine,
				&user.FollowingOfMine,
			)
		}

		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan user: %v", err)
		}

		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate over users: %v", err)
	}

	return users, nil
}
