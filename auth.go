package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// Claims used in JWT
type Claims struct {
	Nickname string  `json:"nickname"`
	Picture  *string `json:"picture,omitempty"`
	jwt.StandardClaims
}

// LoginInput request body
type LoginInput struct {
	Email string `json:"email"`
}

// LoginPayload response body
type LoginPayload struct {
	User             User      `json:"user"`
	IDToken          string    `json:"idToken"`
	RefreshToken     string    `json:"refreshToken"`
	IDTokenExpiresAt time.Time `json:"idTokenExpiresAt"`
}

// ContextKey used in middlewares
type ContextKey int

const (
	keyAuthUserID ContextKey = iota
	keyAuthUser
)

const idTokenLifetime = time.Hour

var jwtKey = []byte(env("JWT_KEY", "secret"))

func jwtKeyfunc(*jwt.Token) (interface{}, error) {
	return jwtKey, nil
}

// Validate user input
func (input *LoginInput) Validate() map[string]string {
	// TODO: actual validation
	return nil
}

// TODO: passwordless
func login(w http.ResponseWriter, r *http.Request) {
	var input LoginInput
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

	var user User
	if err := db.QueryRowContext(r.Context(), `
		SELECT id, username, avatar_url
		FROM users
		WHERE email = $1
	`, email).Scan(
		&user.ID,
		&user.Username,
		&user.AvatarURL,
	); err == sql.ErrNoRows {
		http.Error(w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	} else if err != nil {
		respondError(w, fmt.Errorf("could not query user to login: %v", err))
		return
	}

	idTokenExpiresAt := time.Now().Add(idTokenLifetime)
	idTokenString, err := createIDToken(user, idTokenExpiresAt.Unix())
	if err != nil {
		respondError(w, fmt.Errorf("could not create id token: %v", err))
		return
	}
	refreshTokenString, err := createRefreshToken(user.ID)
	if err != nil {
		respondError(w, fmt.Errorf("could not create refresh token: %v", err))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "id_token",
		Value:    idTokenString,
		Path:     "/",
		Expires:  idTokenExpiresAt,
		HttpOnly: true,
		// Secure:   true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshTokenString,
		Path:     "/",
		HttpOnly: true,
		// Secure:   true,
	})
	respondJSON(w, LoginPayload{
		user,
		idTokenString,
		refreshTokenString,
		idTokenExpiresAt,
	}, http.StatusOK)
}

func logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "id_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		// Secure:   true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		// Secure:   true,
	})
	w.WriteHeader(http.StatusNoContent)
}

func auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idTokenString := r.Header.Get("X-Id-Token")
		refreshTokenString := r.Header.Get("X-Refresh-Token")
		if idTokenString == "" {
			if c, err := r.Cookie("id_token"); err == nil {
				idTokenString = c.Value
			}
		}
		if refreshTokenString == "" {
			c, err := r.Cookie("refresh_token")
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			refreshTokenString = c.Value
		}

		ctx := r.Context()
		p := jwt.Parser{ValidMethods: []string{jwt.SigningMethodHS256.Name}}

		refreshTokenFunc := func() {
			refreshToken, err := p.ParseWithClaims(
				refreshTokenString,
				&jwt.StandardClaims{},
				jwtKeyfunc)
			if err != nil {
				unauthorize(w)
				return
			}
			standardClaims, ok := refreshToken.Claims.(*jwt.StandardClaims)
			if !ok {
				unauthorize(w)
				return
			}

			user := User{ID: standardClaims.Subject}
			if err := db.QueryRowContext(ctx, `
				SELECT username, avatar_url FROM users WHERE id = $1
			`, user.ID).Scan(&user.Username, &user.AvatarURL); err == sql.ErrNoRows {
				http.Error(w, http.StatusText(http.StatusTeapot), http.StatusTeapot)
				return
			} else if err != nil {
				respondError(w, fmt.Errorf("could not fetch user: %v", err))
				return
			}

			idTokenExpiresAt := time.Now().Add(idTokenLifetime)
			idTokenExpiresAtText, err := idTokenExpiresAt.MarshalText()
			if err != nil {
				respondError(w, fmt.Errorf("could not marshall time as text: %v", err))
				return
			}
			if idTokenString, err = createIDToken(user, idTokenExpiresAt.Unix()); err != nil {
				respondError(w, fmt.Errorf("could not create id token: %v", err))
				return
			}

			h := w.Header()
			h.Set("X-Id-Token", idTokenString)
			h.Set("X-Id-Token-Expires-At", string(idTokenExpiresAtText))
			http.SetCookie(w, &http.Cookie{
				Name:     "id_token",
				Value:    idTokenString,
				Path:     "/",
				Expires:  idTokenExpiresAt,
				HttpOnly: true,
				// Secure:   true,
			})

			ctx = context.WithValue(ctx, keyAuthUserID, user.ID)
			ctx = context.WithValue(ctx, keyAuthUser, user)

			next.ServeHTTP(w, r.WithContext(ctx))
		}

		idToken, err := p.ParseWithClaims(idTokenString, &Claims{}, jwtKeyfunc)
		if err != nil {
			refreshTokenFunc()
			return
		}

		claims, ok := idToken.Claims.(*Claims)
		if !ok || !idToken.Valid {
			refreshTokenFunc()
			return
		}

		ctx = context.WithValue(ctx, keyAuthUserID, claims.StandardClaims.Subject)
		ctx = context.WithValue(ctx, keyAuthUser, User{
			ID:        claims.StandardClaims.Subject,
			Username:  claims.Nickname,
			AvatarURL: claims.Picture,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userRequired(next http.Handler) http.Handler {
	return auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, authenticated := r.Context().Value(keyAuthUser).(User); !authenticated {
			unauthorize(w)
			return
		}

		next.ServeHTTP(w, r)
	}))
}

func createIDToken(user User, expiresAt int64) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		user.Username,
		user.AvatarURL,
		jwt.StandardClaims{
			Subject:   user.ID,
			ExpiresAt: expiresAt,
		},
	}).SignedString(jwtKey)
}

func createRefreshToken(userID string) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		Subject: userID,
	}).SignedString(jwtKey)
}

func unauthorize(w http.ResponseWriter) {
	http.Error(w,
		http.StatusText(http.StatusUnauthorized),
		http.StatusUnauthorized)
}
