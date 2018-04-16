package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/lib/pq"
)

var db *sql.DB
var smtpAddress string
var smtpAuth smtp.Auth
var feedBroker *FeedBroker
var commentsBroker *CommentsBroker
var notificationsBroker *NotificationsBroker

func main() {
	var databaseURL, addr, smtpHost, smtpUsername, smtpPassword string
	flag.StringVar(&databaseURL, "crdb",
		env("DATABASE_URL", "postgresql://root@127.0.0.1:26257/nakama?sslmode=disable"),
		"CockroachDB address")
	flag.StringVar(&addr, "http", ":"+env("PORT", "80"), "HTTP address")
	flag.StringVar(&smtpHost, "smtphost", env("SMTP_HOST", "smtp.mailtrap.io"), "SMTP host")
	flag.StringVar(&smtpUsername, "smtpuser", os.Getenv("SMTP_USERNAME"), "SMTP username")
	flag.StringVar(&smtpPassword, "smtppwd", os.Getenv("SMTP_PASSWORD"), "SMTP password")
	flag.Parse()

	if smtpUsername == "" {
		log.Fatal("SMTP username required")
	}
	if smtpPassword == "" {
		log.Fatal("SMTP password required")
	}

	var err error
	db, err = sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("could not open database connection: %v\n", err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatalf("could not ping to database: %v\n", err)
	}

	smtpAddress = net.JoinHostPort(smtpHost, "25")
	smtpAuth = smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)

	feedBroker = newFeedBroker()
	defer close(feedBroker.Notifier)
	commentsBroker = newCommentsBroker()
	defer close(commentsBroker.Notifier)
	notificationsBroker = newNotificationsBroker()
	defer close(notificationsBroker.Notifier)

	mux := chi.NewMux()
	mux.Use(middleware.Recoverer)
	mux.Route("/api", func(api chi.Router) {
		jsonRequired := middleware.AllowContentType("application/json")
		imageRequired := middleware.AllowContentType("image/jpg", "image/jpeg", "image/png")
		api.With(jsonRequired).Post("/passwordless/start", passwordlessStart)
		api.Get("/passwordless/verify_redirect", passwordlessVerifyRedirect)
		api.Post("/logout", logout)
		api.With(mustAuthUser).Get("/me", getMe)
		api.With(jsonRequired).Post("/users", createUser)
		api.With(maybeAuthUserID).Get("/users", getUsers)
		api.With(maybeAuthUserID).Get("/users/{username}", getUser)
		api.With(imageRequired, mustAuthUser).Post("/upload_avatar", uploadAvatar)
		api.With(mustAuthUser).Post("/users/{username}/toggle_follow", toggleFollow)
		api.With(maybeAuthUserID).Get("/users/{username}/followers", getFollowers)
		api.With(maybeAuthUserID).Get("/users/{username}/following", getFollowing)
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
	mux.Get("/favicon.ico", serveFile("static/favicon.ico"))
	mux.Group(func(mux chi.Router) {
		// TODO: remove no cache
		mux.Use(middleware.NoCache)
		mux.Method(http.MethodGet, "/avatars/*", http.StripPrefix("/avatars/", http.FileServer(http.Dir("avatars"))))
		mux.Method(http.MethodGet, "/js/*", http.FileServer(http.Dir("static")))
		mux.Get("/styles.css", serveFile("static/styles.css"))
		mux.Get("/*", serveFile("static/index.html"))
	})

	s := http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: time.Second * 5,
		IdleTimeout:       time.Second * 60,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			log.Fatalf("could not shutdown server: %v\n", err)
		}
	}()

	log.Printf("starting HTTP server at %s", addr)
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

func templateToString(filename string, data interface{}) (string, error) {
	t, err := template.ParseFiles(filename)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	if err := t.Execute(buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func sendMail(subject, to, body string) error {
	from := "noreply@nakama.dev"

	msg := fmt.Sprintf("From: %s\r\n", from)
	msg += fmt.Sprintf("To: %s\r\n", to)
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += "Content-Type: text/html; charset=\"utf-8\"\r\n"
	msg += "\r\n"
	msg += body

	return smtp.SendMail(smtpAddress, smtpAuth, from, []string{to}, []byte(msg))
}
