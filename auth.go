package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// PasswordlessStartInput request body
type PasswordlessStartInput struct {
	Email       string  `json:"email"`
	RedirectURI *string `json:"redirectUri,omitempty"`
}

// ContextKey used in middlewares
type ContextKey int

const (
	keyAuthUserID ContextKey = iota
	keyAuthUser
)

const jwtLifetime = time.Hour * 24 * 60 // 60 days

var jwtKey = []byte(env("JWT_KEY", "secret"))

func jwtKeyFunc(*jwt.Token) (interface{}, error) {
	return jwtKey, nil
}

// Validate request body
func (input *PasswordlessStartInput) Validate() map[string]string {
	// TODO: actual validation
	return nil
}

func passwordlessStart(w http.ResponseWriter, r *http.Request) {
	var input PasswordlessStartInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if errs := input.Validate(); len(errs) != 0 {
		respondJSON(w, errs, http.StatusUnprocessableEntity)
		return
	}

	expiresAt := time.Now().Add(time.Minute * 5)
	var code string
	if err := db.QueryRowContext(r.Context(), `
		INSERT INTO verification_codes (user_id, expires_at) VALUES
			((SELECT id FROM users WHERE email = $1), $2)
		RETURNING code
	`, input.Email, expiresAt).Scan(&code); err == sql.ErrNoRows {
		http.Error(w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	} else if err != nil {
		respondError(w, fmt.Errorf("could not insert verification code: %v", err))
		return
	}

	magicLink, _ := url.Parse("http://localhost/api/passwordless/verify_redirect")
	q := make(url.Values)
	q.Set("email", input.Email)
	q.Set("verification_code", code)
	if input.RedirectURI != nil {
		q.Set("redirect_uri", *input.RedirectURI)
	}
	magicLink.RawQuery = q.Encode()

	body, err := templateToString("templates/magic-link.html", map[string]string{
		"magicLink": magicLink.String(),
	})
	if err != nil {
		respondError(w, fmt.Errorf("could not build magic link template: %v", err))
		return
	}

	if err := sendMail("Magic Link", input.Email, body); err != nil {
		log.Printf("could not send magic link: %v\n", err)
		http.Error(w,
			http.StatusText(http.StatusServiceUnavailable),
			http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func passwordlessVerifyRedirect(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	email := q.Get("email")
	verificationCode := q.Get("verification_code")
	redirectURIString := q.Get("redirect_uri")
	if redirectURIString == "" {
		redirectURIString = "http://localhost/callback"
	}
	redirectURI, err := url.Parse(redirectURIString)
	if err != nil {
		respondJSON(w,
			map[string]string{"redirectUri": "Invalid redirect URI"},
			http.StatusUnprocessableEntity)
		return
	}

	var userID string
	if err := db.QueryRowContext(r.Context(), `
		DELETE FROM verification_codes
		WHERE code = $1
			AND user_id = (SELECT id FROM users WHERE email = $2)
			AND expires_at > now()
		RETURNING user_id`, verificationCode, email).Scan(&userID); err == sql.ErrNoRows {
		http.Error(w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	} else if err != nil {
		respondError(w, fmt.Errorf("could not delete verification code: %v", err))
		return
	}

	expiresAt := time.Now().Add(jwtLifetime)
	expiresAtBytes, _ := expiresAt.MarshalText()
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		Subject:   userID,
		ExpiresAt: expiresAt.Unix(),
	}).SignedString(jwtKey)
	if err != nil {
		respondError(w, fmt.Errorf("could not create jwt: %v", err))
		return
	}

	fragment := make(url.Values)
	fragment.Set("jwt", tokenString)
	fragment.Set("expires_at", string(expiresAtBytes))
	redirectURI.Fragment = fragment.Encode()

	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    tokenString,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		// Secure:   true,
	})
	http.Redirect(w, r, redirectURI.String(), http.StatusFound)
}

func logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		// Secure:   true,
	})
	w.WriteHeader(http.StatusNoContent)
}

func getMe(w http.ResponseWriter, r *http.Request) {
	authUser := r.Context().Value(keyAuthUser).(User)
	respondJSON(w, authUser, http.StatusOK)
}

func maybeAuthUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenString string
		if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
			tokenString = a[7:]
		} else if c, err := r.Cookie("jwt"); err == nil {
			tokenString = c.Value
		} else {
			next.ServeHTTP(w, r)
			return
		}

		p := jwt.Parser{ValidMethods: []string{jwt.SigningMethodHS256.Name}}
		token, err := p.ParseWithClaims(tokenString, &jwt.StandardClaims{}, jwtKeyFunc)
		if err != nil {
			unauthorize(w)
			return
		}

		claims, ok := token.Claims.(*jwt.StandardClaims)
		if !ok || !token.Valid {
			unauthorize(w)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, keyAuthUserID, claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func mustAuthUser(next http.Handler) http.Handler {
	return maybeAuthUserID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		authUserID, authenticated := ctx.Value(keyAuthUserID).(string)
		if !authenticated {
			unauthorize(w)
			return
		}

		var user User
		if err := db.QueryRowContext(ctx, `
			SELECT username, avatar_url FROM users WHERE id = $1
		`, authUserID).Scan(&user.Username, &user.AvatarURL); err == sql.ErrNoRows {
			http.Error(w,
				http.StatusText(http.StatusTeapot),
				http.StatusTeapot)
			return
		} else if err != nil {
			respondError(w, fmt.Errorf("could not query auth user: %v", err))
			return
		}

		user.ID = authUserID
		ctx = context.WithValue(ctx, keyAuthUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}

func unauthorize(w http.ResponseWriter) {
	http.Error(w,
		http.StatusText(http.StatusUnauthorized),
		http.StatusUnauthorized)
}
