package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kurt4ins/taskmanager/internal/repo"
	"github.com/kurt4ins/taskmanager/internal/utils"
	"golang.org/x/sync/errgroup"
)

type ReportHandler struct {
	taskRepo repo.TaskRepository
	userRepo repo.UserRepository
}

func NewReportHandler(tr repo.TaskRepository, ur repo.UserRepository) *ReportHandler {
	return &ReportHandler{taskRepo: tr, userRepo: ur}
}

func slowWork(ctx context.Context) (map[string]string, error) {
	select {
	case <-time.After(5 * time.Second):
		return map[string]string{"data": "report done"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (h *ReportHandler) SlowReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	res, err := slowWork(ctx)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Println("client has disconnected")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "report failed")
		return
	}

	utils.WriteJSON(w, http.StatusOK, res)
}

func (h *ReportHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	g, ctx := errgroup.WithContext(r.Context())

	var tasks []repo.Task
	var users []repo.User

	g.Go(func() error {
		var err error
		tasks, err = h.taskRepo.List(ctx, 10, 0)
		return err
	})

	g.Go(func() error {
		var err error
		users, err = h.userRepo.List(ctx, 10, 0)
		return err
	})

	// g.Go(func() error {
	// 	select {
	// 	case <-time.After(5 * time.Second):
	// 		return fmt.Errorf("fail")
	// 	case <-ctx.Done():
	// 		return ctx.Err()
	// 	}
	// })

	if err := g.Wait(); err != nil {
		if r.Context().Err() != nil { // уточнить срабатывает ли это только при отключении клиента
			fmt.Println("client has disconnected")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "dashboard failed")
		return
	}

	res := map[string]any{"tasks": tasks, "users": users}

	utils.WriteJSON(w, http.StatusOK, res)
}
