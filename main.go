package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/lib/pq"
)

var db *sql.DB

func main() {
	var err error
	databaseURL := env("DATABASE_URL", "postgresql://root@127.0.0.1:26257/nakama?sslmode=disable")
	db, err = sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("could not open database connection: %v\n", err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatalf("could not ping to database: %v\n", err)
	}

	mux := chi.NewMux()
	mux.Use(middleware.Recoverer)
	mux.Route("/api", func(api chi.Router) {
		jsonRequired := middleware.AllowContentType("application/json")
		api.With(jsonRequired).Post("/login", login)
		api.Post("/logout", logout)
		api.With(jsonRequired).Post("/users", createUser)
		api.With(maybeAuthUserID).Get("/users", getUsers)
		api.With(maybeAuthUserID).Get("/users/{username}", getUser)
		api.With(mustAuthUser).Post("/users/{username}/toggle_follow", toggleFollow)
		api.With(jsonRequired, mustAuthUser).Post("/posts", createPost)
		api.With(maybeAuthUserID).Get("/users/{username}/posts", getPosts)
		api.With(maybeAuthUserID).Get("/posts/{post_id}", getPost)
		api.With(mustAuthUser).Get("/feed", getFeed)
		api.With(jsonRequired, mustAuthUser).Post("/posts/{post_id}/comments", createComment)
		api.With(maybeAuthUserID).Get("/posts/{post_id}/comments", getComments)
		api.With(mustAuthUser).Post("/posts/{post_id}/toggle_like", togglePostLike)
		api.With(mustAuthUser).Post("/posts/{post_id}/toggle_subscription", toggleSubscription)
		api.With(mustAuthUser).Post("/comments/{comment_id}/toggle_like", toggleCommentLike)
		api.With(mustAuthUser).Get("/notifications", getNotifications)
		api.With(mustAuthUser).Get("/check_unread_notifications", checkUnreadNotifications)
	})
	mux.Group(func(mux chi.Router) {
		// TODO: remove no cache
		mux.Use(middleware.NoCache)
		mux.Get("/js/*", http.FileServer(http.Dir("static")).ServeHTTP)
		mux.Get("/styles.css", serveFile("static/styles.css"))
		mux.Get("/*", serveFile("static/index.html"))
	})

	port := env("PORT", "80")
	s := http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			log.Fatalf("could not shutdown the server: %v\n", err)
		}
	}()

	log.Printf("Server listenning on port %s", port)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("could not start server: %v\n", err)
	}
}

func env(key, fallbackValue string) string {
	value, present := os.LookupEnv(key)
	if !present {
		return fallbackValue
	}
	return value
}

func respondError(w http.ResponseWriter, err error) {
	log.Println(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func respondJSON(w http.ResponseWriter, v interface{}, code int) {
	b, err := json.Marshal(v)
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(b)
}

func serveFile(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, name)
	}
}
