package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kurt4ins/taskmanager/internal/handlers"
	"github.com/kurt4ins/taskmanager/internal/repo"
)

func connectDB() *sql.DB {
	db, err := sql.Open("pgx", "postgres://taskmanager:taskmanager@db:5432/taskmanager?sslmode=disable")
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
	db := connectDB()
	defer db.Close()

	taskRepo := repo.NewTaskRepo(db)
	userRepo := repo.NewUserRepo(db)

	mux := http.NewServeMux()

	taskH := handlers.NewTaskHandler(taskRepo)
	userH := handlers.NewUserHandler(userRepo)

	mux.HandleFunc("POST /tasks", taskH.Create)
	mux.HandleFunc("GET /tasks/{id}", taskH.GetById)
	mux.HandleFunc("GET /tasks", taskH.List)

	mux.HandleFunc("POST /users", userH.Create)
	mux.HandleFunc("GET /users/{id}", userH.GetById)
	mux.HandleFunc("GET /users", userH.List)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]string{"status": "ok"}
		handlers.WriteJSON(w, http.StatusOK, data)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handlers.WriteError(w, http.StatusNotFound, "page not found")
	})

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Printf("starting server at %s\n", server.Addr)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println("failed to start server")
		fmt.Println(err)
	}
}
