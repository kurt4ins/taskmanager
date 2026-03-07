package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Task struct {
	Id          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

type TaskRepository interface {
	Create(ctx context.Context, task Task) (*Task, error)
	GetById(ctx context.Context, id int) (*Task, error)
	List(ctx context.Context, limit int, offset int) ([]Task, error)
}

type TaskRepo struct {
	db *sql.DB
}

func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) Create(ctx context.Context, task Task) (*Task, error) {
	query := `
		insert into tasks (title, description, completed)
		values ($1, $2, $3)
		returning id, title, description, completed
	`

	err := r.db.QueryRowContext(ctx, query, task.Title, task.Description, task.Completed).
		Scan(&task.Id, &task.Title, &task.Description, &task.Completed)
	if err != nil {
		return nil, fmt.Errorf("TaskRepo.Create: %w", err)
	}

	return &task, nil
}

func (r *TaskRepo) GetById(ctx context.Context, id int) (*Task, error) {
	query := `select title, description, completed from tasks where id = $1`

	task := Task{Id: id}
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&task.Title, &task.Description, &task.Completed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("TaskRepo.GetById: %w", err)
	}

	return &task, nil
}

func (r *TaskRepo) List(ctx context.Context, limit int, offset int) ([]Task, error) {
	const maxLimit = 100
	if limit > maxLimit {
		limit = maxLimit
	}

	query := `
		select id, title, description, completed from tasks
		order by id
		limit $1 offset $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("TaskRepo.List: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.Id, &task.Title, &task.Description, &task.Completed); err != nil {
			return nil, fmt.Errorf("TaskRepo.List scan: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("TaskRepo.List rows: %w", err)
	}

	return tasks, nil
}
