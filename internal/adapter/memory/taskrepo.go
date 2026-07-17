package memory

import (
	"context"
	"sync"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
)

// TaskRepository is a mutex-guarded map of tasks by ID.
type TaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]domain.Task
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{tasks: make(map[string]domain.Task)}
}

func (r *TaskRepository) Save(_ context.Context, t domain.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[t.ID] = t
	return nil
}

func (r *TaskRepository) Get(_ context.Context, id string) (domain.Task, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tasks[id]
	return t, ok
}
