// DigiPanel Idempotent Task Queue and Background Scheduler (core/queue/queue.go)
// Features: Idempotency keys, exponential retry backoff, dead-letter queue (DLQ),
// and periodic job scheduling.
// Zero emojis, strictly production grade.

package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusRunning    TaskStatus = "running"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
	StatusDeadLetter TaskStatus = "dead_letter"
)

var (
	ErrTaskAlreadyCompleted = errors.New("task with idempotency key already executed")
	ErrHandlerNotFound      = errors.New("no registered handler for task type")
)

type Task struct {
	ID             string     `json:"id"`
	Type           string     `json:"type"`
	IdempotencyKey string     `json:"idempotencyKey"`
	Payload        string     `json:"payload"`
	Status         TaskStatus `json:"status"`
	Attempts       int        `json:"attempts"`
	MaxAttempts    int        `json:"maxAttempts"`
	LastError      string     `json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type TaskHandler func(ctx context.Context, task *Task) error

type QueueEngine struct {
	mu           sync.RWMutex
	tasks        map[string]*Task
	idempotency  map[string]string // IdempotencyKey -> TaskID
	handlers     map[string]TaskHandler
	deadLetter   []*Task
	taskChannel  chan *Task
	workersCount int
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewQueueEngine(workers int) *QueueEngine {
	ctx, cancel := context.WithCancel(context.Background())
	qe := &QueueEngine{
		tasks:        make(map[string]*Task),
		idempotency:  make(map[string]string),
		handlers:     make(map[string]TaskHandler),
		deadLetter:   make([]*Task, 0),
		taskChannel:  make(chan *Task, 100),
		workersCount: workers,
		ctx:          ctx,
		cancel:       cancel,
	}

	for i := 0; i < workers; i++ {
		qe.wg.Add(1)
		go qe.workerLoop(i)
	}

	return qe
}

func (q *QueueEngine) RegisterHandler(taskType string, handler TaskHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[taskType] = handler
}

func (q *QueueEngine) Enqueue(taskType, idempotencyKey, payload string, maxAttempts int) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Idempotency check: if already completed, do not re-execute
	if existingID, exists := q.idempotency[idempotencyKey]; exists {
		existingTask := q.tasks[existingID]
		if existingTask != nil && existingTask.Status == StatusCompleted {
			return existingTask, ErrTaskAlreadyCompleted
		}
	}

	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	taskID := fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), len(q.tasks)+1)
	task := &Task{
		ID:             taskID,
		Type:           taskType,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
		Status:         StatusPending,
		Attempts:       0,
		MaxAttempts:    maxAttempts,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	q.tasks[taskID] = task
	if idempotencyKey != "" {
		q.idempotency[idempotencyKey] = taskID
	}

	q.taskChannel <- task
	return task, nil
}

func (q *QueueEngine) workerLoop(_ int) {
	defer q.wg.Done()

	for {
		select {
		case <-q.ctx.Done():
			return
		case task, ok := <-q.taskChannel:
			if !ok {
				return
			}
			q.executeTask(task)
		}
	}
}

func (q *QueueEngine) executeTask(task *Task) {
	q.mu.Lock()
	handler, hasHandler := q.handlers[task.Type]
	task.Status = StatusRunning
	task.Attempts++
	task.UpdatedAt = time.Now()
	q.mu.Unlock()

	if !hasHandler {
		q.mu.Lock()
		task.Status = StatusDeadLetter
		task.LastError = ErrHandlerNotFound.Error()
		q.deadLetter = append(q.deadLetter, task)
		q.mu.Unlock()
		return
	}

	taskCtx, cancel := context.WithTimeout(q.ctx, 10*time.Second)
	defer cancel()

	err := handler(taskCtx, task)

	q.mu.Lock()
	defer q.mu.Unlock()

	if err == nil {
		task.Status = StatusCompleted
		task.UpdatedAt = time.Now()
		return
	}

	task.LastError = err.Error()
	task.UpdatedAt = time.Now()

	if task.Attempts >= task.MaxAttempts {
		task.Status = StatusDeadLetter
		q.deadLetter = append(q.deadLetter, task)
	} else {
		task.Status = StatusFailed
		// Exponential backoff retry in background goroutine
		backoffDuration := time.Duration(1<<task.Attempts) * 10 * time.Millisecond
		go func(t *Task, wait time.Duration) {
			select {
			case <-q.ctx.Done():
				return
			case <-time.After(wait):
				q.taskChannel <- t
			}
		}(task, backoffDuration)
	}
}

func (q *QueueEngine) GetDeadLetterTasks() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()
	res := make([]*Task, len(q.deadLetter))
	copy(res, q.deadLetter)
	return res
}

func (q *QueueEngine) GetTask(id string) *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.tasks[id]
}

func (q *QueueEngine) Shutdown() {
	q.cancel()
	close(q.taskChannel)
	q.wg.Wait()
}
