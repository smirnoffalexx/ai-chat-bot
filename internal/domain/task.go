package domain

import "time"

// TaskStatus is the lifecycle state of an agent task.
type TaskStatus string

const (
	TaskQueued  TaskStatus = "queued"
	TaskRunning TaskStatus = "running"
	TaskDone    TaskStatus = "done"
	TaskFailed  TaskStatus = "failed"
)

// Task represents a unit of work the agent must solve for a user. A task may be
// answered immediately or take a while; either way it is processed off the main
// polling loop so slow work never blocks the bot.
type Task struct {
	ID        string
	ChatID    ChatID
	UserID    UserID
	UserName  string
	Prompt    string
	Status    TaskStatus
	Result    string
	Err       string
	CreatedAt time.Time
	StartedAt time.Time
	EndedAt   time.Time
}
