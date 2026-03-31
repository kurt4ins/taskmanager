package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Task struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	WordCount   *int   `json:"word_count"`
}

type PatchTask struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Completed   *bool   `json:"completed"`
}

type TaskRepository interface {
	Create(ctx context.Context, task Task) (*Task, error)
	GetById(ctx context.Context, id int) (*Task, error)
	List(ctx context.Context, limit int, offset int) ([]Task, error)
	ListByUser(ctx context.Context, userId int, limit int, offset int) ([]Task, error)
	Update(ctx context.Context, task Task) (*Task, error)
	Patch(ctx context.Context, id int, fields PatchTask) (*Task, error)
	Delete(ctx context.Context, id int) error
}

type TaskRepo struct {
	db    *sql.DB
	audit *AuditLogRepo
}

func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db, audit: NewAuditLogRepo(db)}
}

func (r *TaskRepo) Create(ctx context.Context, task Task) (*Task, error) {
	query := `
		insert into tasks (user_id, title, description, completed)
		values ($1, $2, $3, $4)
		returning id, user_id, title, description, completed, word_count
	`

	err := r.db.QueryRowContext(ctx, query, task.UserId, task.Title, task.Description, task.Completed).
		Scan(&task.Id, &task.UserId, &task.Title, &task.Description, &task.Completed, &task.WordCount)
	if err != nil {
		return nil, fmt.Errorf("TaskRepo.Create: %w", err)
	}

	return &task, nil
}

func (r *TaskRepo) GetById(ctx context.Context, id int) (*Task, error) {
	query := `select user_id, title, description, completed, word_count from tasks where id = $1`

	task := Task{Id: id}
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&task.UserId, &task.Title, &task.Description, &task.Completed, &task.WordCount)
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
		select id, user_id, title, description, completed, word_count from tasks
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
		if err := rows.Scan(&task.Id, &task.UserId, &task.Title, &task.Description, &task.Completed, &task.WordCount); err != nil {
			return nil, fmt.Errorf("TaskRepo.List scan: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("TaskRepo.List rows: %w", err)
	}

	return tasks, nil
}

func (r *TaskRepo) Update(ctx context.Context, task Task) (*Task, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("TaskRepo.Update: %w", err)
	}
	defer tx.Commit()

	var oldTask Task
	query := `select id, user_id, title, description, completed, word_count from tasks where id = $1`

	err = tx.QueryRowContext(ctx, query, task.Id).
		Scan(&oldTask.Id, &oldTask.UserId, &oldTask.Title, &oldTask.Description, &oldTask.Completed, &oldTask.WordCount)

	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("TaskRepo.Update fetch old: %w", err)
	}

	query = `
		update tasks
		set title = $1, description = $2, completed = $3
		where id = $4
		returning id, user_id, title, description, completed, word_count
	`

	err = tx.QueryRowContext(ctx, query, task.Title, task.Description, task.Completed, task.Id).
		Scan(&task.Id, &task.UserId, &task.Title, &task.Description, &task.Completed, &task.WordCount)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("TaskRepo.Update: %w", err)
	}

	if err = r.audit.Log(ctx, tx, AuditLog{
		Entity:   "task",
		EntityId: task.Id,
		Action:   "update",
		OldData:  toJSON(oldTask),
		NewData:  toJSON(task),
	}); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("TaskRepo.Update audit: %w", err)
	}

	return &task, nil
}

func (r *TaskRepo) Patch(ctx context.Context, id int, fields PatchTask) (*Task, error) {
	query := `
		update tasks
		set title = coalesce($1, title),
		    description = coalesce($2, description),
		    completed = coalesce($3, completed)
		where id = $4
		returning id, user_id, title, description, completed, word_count
	`

	var task Task
	err := r.db.QueryRowContext(ctx, query, fields.Title, fields.Description, fields.Completed, id).
		Scan(&task.Id, &task.UserId, &task.Title, &task.Description, &task.Completed, &task.WordCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("TaskRepo.Patch: %w", err)
	}

	return &task, nil
}

func (r *TaskRepo) Delete(ctx context.Context, id int) error {
	query := `delete from tasks where id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("TaskRepo.Delete: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("TaskRepo.Delete rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("TaskRepo.Delete: not found")
	}

	return nil
}

func (r *TaskRepo) ListByUser(ctx context.Context, userId int, limit int, offset int) ([]Task, error) {
	const maxLimit = 100
	if limit > maxLimit {
		limit = maxLimit
	}

	query := `
		select id, user_id, title, description, completed, word_count from tasks
		where user_id = $1
		order by id
		limit $2 offset $3
	`

	rows, err := r.db.QueryContext(ctx, query, userId, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("TaskRepo.ListByUser: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.Id, &task.UserId, &task.Title, &task.Description, &task.Completed, &task.WordCount); err != nil {
			return nil, fmt.Errorf("TaskRepo.ListByUser scan: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("TaskRepo.ListByUser rows: %w", err)
	}

	return tasks, nil
}

func (r *TaskRepo) UpdateWordCount(ctx context.Context, taskID int, wordCount int) error {
	_, err := r.db.ExecContext(ctx,
		`update tasks set word_count = $1 where id = $2`,
		wordCount, taskID)
	if err != nil {
		return fmt.Errorf("TaskRepo.UpdateWordCount: %w", err)
	}
	return nil
}
