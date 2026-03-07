package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type RequestUser struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type User struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserRepository interface {
	Create(ctx context.Context, reqUser RequestUser) (*User, error)
	GetById(ctx context.Context, id int) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, limit int, offset int) ([]User, error)
	// Update()
	// Patch()
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, reqUser RequestUser) (*User, error) {
	query := `
		insert into users (name, email, password)
		values ($1, $2, $3)
		returning id, name, email
	`

	password, err := bcrypt.GenerateFromPassword([]byte(reqUser.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("password hashing error: %w\n", err)
	}

	var user User
	err = r.db.QueryRowContext(ctx, query, reqUser.Name, reqUser.Email, password).
		Scan(&user.Id, &user.Name, &user.Email)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.Create: %w", err)
	}

	return &user, nil
}

func (r *UserRepo) GetById(ctx context.Context, id int) (*User, error) {
	query := `select name, email from users where id = $1`
	user := User{Id: id}

	err := r.db.QueryRowContext(ctx, query, id).Scan(&user.Name, &user.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.GetById: %w", err)
	}

	return &user, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `select id, name from users where email = $1`
	user := User{Email: email}

	err := r.db.QueryRowContext(ctx, query, email).Scan(&user.Id, &user.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("UserRepo.GetByEmail: %w", err)
	}

	return &user, nil
}

func (r *UserRepo) List(ctx context.Context, limit int, offset int) ([]User, error) {
	const maxLimit = 100
	if limit > maxLimit {
		limit = maxLimit
	}

	query := `
		select id, name, email from users
		order by id
		limit $1 offset $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.List: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Id, &user.Name, &user.Email); err != nil {
			return nil, fmt.Errorf("UserRepo.List scan: %w", err)
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("UserRepo.List rows: %w", err)
	}

	return users, nil
}
