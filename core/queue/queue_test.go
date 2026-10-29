// DigiPanel Task Queue Unit Tests (core/queue/queue_test.go)
// Verifies worker execution, idempotency, retry backoff, and dead-letter queue.

package queue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueueSuccessfulExecution(t *testing.T) {
	qe := NewQueueEngine(2)
	defer qe.Shutdown()

	var executed int32
	qe.RegisterHandler("test.job", func(ctx context.Context, task *Task) error {
		atomic.AddInt32(&executed, 1)
		return nil
	})

	task, err := qe.Enqueue("test.job", "idem-key-1", `{"foo":"bar"}`, 3)
	if err != nil {
		t.Fatalf("Failed to enqueue task: %v", err)
	}

	// Wait for execution
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("Expected task to be executed once, got %d", executed)
	}

	refreshed := qe.GetTask(task.ID)
	if refreshed.Status != StatusCompleted {
		t.Errorf("Expected task status 'completed', got '%s'", refreshed.Status)
	}
}

func TestQueueIdempotencyKeyEnforcement(t *testing.T) {
	qe := NewQueueEngine(2)
	defer qe.Shutdown()

	var counter int32
	qe.RegisterHandler("idempotent.job", func(ctx context.Context, task *Task) error {
		atomic.AddInt32(&counter, 1)
		return nil
	})

	// First execution
	_, err := qe.Enqueue("idempotent.job", "unique-payment-101", `{"charge":50000}`, 3)
	if err != nil {
		t.Fatalf("Initial enqueue failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Second execution with identical idempotency key
	_, errSecond := qe.Enqueue("idempotent.job", "unique-payment-101", `{"charge":50000}`, 3)
	if !errors.Is(errSecond, ErrTaskAlreadyCompleted) {
		t.Errorf("Expected ErrTaskAlreadyCompleted, got: %v", errSecond)
	}

	if atomic.LoadInt32(&counter) != 1 {
		t.Errorf("Security/Integrity failure: idempotent task was executed multiple times (%d)", counter)
	}
}

func TestQueueDeadLetterQueueOnExceededAttempts(t *testing.T) {
	qe := NewQueueEngine(2)
	defer qe.Shutdown()

	var attempts int32
	qe.RegisterHandler("failing.job", func(ctx context.Context, task *Task) error {
		atomic.AddInt32(&attempts, 1)
		return errors.New("upstream service unavailable")
	})

	task, err := qe.Enqueue("failing.job", "failing-key-1", `{}`, 2) // max 2 attempts
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	// Wait for retries
	time.Sleep(150 * time.Millisecond)

	refreshed := qe.GetTask(task.ID)
	if refreshed.Status != StatusDeadLetter {
		t.Errorf("Expected task to end in dead_letter, got '%s'", refreshed.Status)
	}

	deadLetters := qe.GetDeadLetterTasks()
	if len(deadLetters) != 1 {
		t.Errorf("Expected 1 dead letter task, got %d", len(deadLetters))
	}
}
