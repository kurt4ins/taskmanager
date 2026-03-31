package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/caarlos0/env/v11"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kurt4ins/taskmanager/internal/handlers"
	"github.com/kurt4ins/taskmanager/internal/indexer"
	"github.com/kurt4ins/taskmanager/internal/middleware"
	"github.com/kurt4ins/taskmanager/internal/repo"
	"github.com/kurt4ins/taskmanager/internal/utils"
)

type config struct {
	JWTSecret   string `env:"JWT_SECRET,required"`
	DatabaseURL string `env:"DATABASE_URL,required"`
}

func connectDB(databaseURL string) *sql.DB {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		panic(err)
	}

	if err := db.Ping(); err != nil {
		panic(err)
	}
	fmt.Println("connected to a database")

	return db
}

func main() {
	var cfg config
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}

	db := connectDB(cfg.DatabaseURL)
	defer db.Close()

	taskRepo := repo.NewTaskRepo(db)
	userRepo := repo.NewUserRepo(db)

	idx := indexer.New(taskRepo, 100, 2)

	mux := http.NewServeMux()

	taskH := handlers.NewTaskHandler(taskRepo, idx)
	userH := handlers.NewUserHandler(userRepo)
	authH := handlers.NewAuthHandler(userRepo, []byte(cfg.JWTSecret))
	reportH := handlers.NewReportHandler(taskRepo, userRepo)

	auth := middleware.Auth([]byte(cfg.JWTSecret))

	mux.HandleFunc("POST /auth/login", authH.Login)
	mux.HandleFunc("POST /auth/refresh", authH.Refresh)

	mux.HandleFunc("GET /users/{userId}/tasks", taskH.ListByUser)
	mux.HandleFunc("GET /tasks/{id}", taskH.GetById)
	mux.HandleFunc("GET /tasks", taskH.List)

	mux.Handle("POST /users/{userId}/tasks", auth(http.HandlerFunc(taskH.Create)))
	mux.Handle("PUT /tasks/{id}", auth(http.HandlerFunc(taskH.Update)))
	mux.Handle("PATCH /tasks/{id}", auth(http.HandlerFunc(taskH.Patch)))
	mux.Handle("DELETE /tasks/{id}", auth(http.HandlerFunc(taskH.Delete)))

	mux.HandleFunc("POST /users", userH.Create)
	mux.HandleFunc("GET /users/{id}", userH.GetById)
	mux.HandleFunc("GET /users", userH.List)

	mux.HandleFunc("GET /reports/slow", reportH.SlowReport)
	mux.HandleFunc("GET /dashboard", reportH.Dashboard)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]string{"status": "ok"}
		utils.WriteJSON(w, http.StatusOK, data)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteError(w, http.StatusNotFound, "page not found")
	})

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Printf("starting server at %s\n", server.Addr)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println("failed to start server")
		fmt.Println(err)
	}
}
