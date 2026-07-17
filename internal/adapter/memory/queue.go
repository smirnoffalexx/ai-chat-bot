// Package memory provides in-memory implementations of the storage and queue
// ports. They are process-local and non-durable; swap for Redis/Postgres-backed
// adapters when persistence across restarts is required.
package memory

import (
	"context"

	"github.com/smirnoffalexx/ai-chat-bot/internal/domain"
)

// Queue is a buffered channel-based job queue.
type Queue struct {
	ch chan domain.Task
}

// NewQueue creates a queue with the given buffer capacity.
func NewQueue(buffer int) *Queue {
	if buffer <= 0 {
		buffer = 64
	}
	return &Queue{ch: make(chan domain.Task, buffer)}
}

func (q *Queue) Enqueue(ctx context.Context, t domain.Task) error {
	select {
	case q.ch <- t:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Queue) Dequeue(ctx context.Context) (domain.Task, error) {
	select {
	case t := <-q.ch:
		return t, nil
	case <-ctx.Done():
		return domain.Task{}, ctx.Err()
	}
}
