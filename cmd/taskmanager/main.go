package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const maxBodyBytes = 1024 * 1024

var nextId = 2

type User struct {
	Id       int
	Name     string
	Email    string
	Password []byte
}

type ApiError struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"request_id,omitempty"`
}

type ErrorResponse struct {
	Error ApiError `json:"error"`
}

var users = []User{
	{
		Id:    1,
		Name:  "Eugene",
		Email: "eugene@gmail.com",
	},
}

func readJSON(w http.ResponseWriter, r *http.Request, data any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)

	if err := dec.Decode(data); err != nil {
		var syntaxErr *json.SyntaxError
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &syntaxErr):
			return fmt.Errorf("syntax error in json %w", err)
		case errors.As(err, &maxErr):
			return fmt.Errorf("request body is too large (maximum 1MB)")
		default:
			return fmt.Errorf("failed to decode JSON: %w", err)
		}
	}

	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body contains invalid JSON: too many objects")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Printf("writeJSON encode error: %v\n", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	response := ErrorResponse{
		Error: ApiError{
			Code:      status,
			Message:   msg,
			RequestId: uuid.New().String(),
		},
	}
	writeJSON(w, status, response)
}

func validateUser(data map[string]string) ([]string, bool) {
	flag := true
	var msg []string

	if _, ok := data["name"]; !ok {
		msg = append(msg, "name wasn't provided")
		flag = false
	}

	if _, ok := data["email"]; !ok {
		msg = append(msg, "email wasn't provided")
		flag = false
	} else if _, err := mail.ParseAddress(data["email"]); err != nil {
		msg = append(msg, "invalid email")
		flag = false
	} else {
		for i := range users {
			if users[i].Email == data["email"] {
				msg = append(msg, fmt.Sprintf("user with email %s already exists", data["email"]))
				flag = false
			}
		}
	}

	if _, ok := data["password"]; !ok {
		msg = append(msg, "password wasn't provided")
		flag = false
	}

	return msg, flag
}

func findUser(id int) (*User, bool) {
	for i := range users {
		if users[i].Id == id {
			return &users[i], true
		}
	}
	return nil, false
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]string{"status": "ok"}
		writeJSON(w, http.StatusOK, data)
	})

	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		var data json.RawMessage
		if err := readJSON(w, r, &data); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, &data)
	})

	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		var data []map[string]string
		for _, user := range users {
			u := map[string]string{"id": strconv.Itoa(user.Id), "name": user.Name, "email": user.Email}
			data = append(data, u)
		}

		writeJSON(w, http.StatusOK, data)
	})

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		strId := r.PathValue("id")
		id, err := strconv.Atoi(strId)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id provided")
			return
		}
		if user, ok := findUser(id); ok {
			data := map[string]string{
				"name":  user.Name,
				"email": user.Email,
			}
			writeJSON(w, http.StatusOK, &data)
		} else {
			writeError(w, http.StatusNotFound, fmt.Sprintf("user with id %d doesn't exist", id))
		}
	})

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var data map[string]string
		if err := readJSON(w, r, &data); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if msg, ok := validateUser(data); !ok {
			writeError(w, http.StatusBadRequest, strings.Join(msg, "; "))
			return
		}

		password, err := bcrypt.GenerateFromPassword([]byte(data["password"]), bcrypt.MinCost)
		if err != nil {
			fmt.Printf("password hashing error: %v\n", err)
		}

		users = append(users, User{
			Id:       nextId,
			Name:     data["name"],
			Email:    data["email"],
			Password: password,
		})
		nextId++

		response := map[string]string{"message": "user created successfully"}
		writeJSON(w, http.StatusCreated, response)
	})

	mux.HandleFunc("PUT /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		strId := r.PathValue("id")
		id, err := strconv.Atoi(strId)
		if err != nil {
			writeError(w, http.StatusNotFound, "invalid id provided")
			return
		}

		var data map[string]string
		if err := readJSON(w, r, &data); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if user, ok := findUser(id); ok {
			password, err := bcrypt.GenerateFromPassword([]byte(data["password"]), bcrypt.MinCost)
			if err != nil {
				fmt.Printf("password hashing error: %v\n", err)
			}
			user.Name = data["name"]
			user.Email = data["email"]
			user.Password = password

			writeJSON(w, http.StatusOK, "user credentials updated successfully")
		} else {
			writeError(w, http.StatusNotFound, fmt.Sprintf("user with id %d doesn't exist", id))
		}

	})

	mux.HandleFunc("PATCH /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		strId := r.PathValue("id")
		id, err := strconv.Atoi(strId)
		if err != nil {
			writeError(w, http.StatusNotFound, "invalid id provided")
			return
		}

		var data map[string]string
		if err := readJSON(w, r, &data); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if user, ok := findUser(id); ok {
			if data["name"] != "" {
				user.Name = data["name"]
			}
			if data["email"] != "" {
				user.Email = data["email"]
			}
			if data["password"] != "" {
				password, err := bcrypt.GenerateFromPassword([]byte(data["password"]), bcrypt.MinCost)
				if err != nil {
					fmt.Printf("password hashing error: %v\n", err)
				}
				user.Password = password
			}

			writeJSON(w, http.StatusOK, "user credentials updated successfully")
		} else {
			writeError(w, http.StatusNotFound, fmt.Sprintf("user with id %d doesn't exist", id))
		}

	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "page not found")
	})

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	fmt.Printf("starting server at %s\n", server.Addr)

	if err := server.ListenAndServe(); err != nil {
		fmt.Println("failed to start server")
		fmt.Println(err)
	}
}
