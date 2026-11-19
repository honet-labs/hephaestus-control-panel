package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-hephaestus/internal/core/domain"
)

func testPool(t *testing.T, workers int) *WorkerPool {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{numWorkers: workers, jobQueue: make(chan *domain.Job, 2), handlers: make(map[string]JobHandlerFunc), jobs: make(map[string]*domain.Job), cancels: make(map[string]context.CancelFunc), ctx: ctx, cancel: cancel, maxJobStore: 10}
	wp.start()
	t.Cleanup(wp.Stop)
	return wp
}

func waitStatus(t *testing.T, wp *WorkerPool, id string, status domain.JobStatus) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		job, _ := wp.GetJob(id)
		if job != nil && job.Status == status {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("job did not reach %s: %+v", status, job)
		case <-tick.C:
		}
	}
}

func TestWorkerPanicDoesNotKillWorker(t *testing.T) {
	wp := testPool(t, 1)
	wp.RegisterHandler("panic", func(context.Context, *domain.Job, func(int, string)) error { panic("test") })
	wp.RegisterHandler("ok", func(context.Context, *domain.Job, func(int, string)) error { return nil })
	job, _ := wp.Enqueue("panic", nil, 0)
	waitStatus(t, wp, job.ID, domain.JobStatusFailed)
	job, _ = wp.Enqueue("ok", nil, 0)
	waitStatus(t, wp, job.ID, domain.JobStatusCompleted)
}

func TestCancelDuringRetryAndStop(t *testing.T) {
	wp := testPool(t, 1)
	called := make(chan struct{}, 2)
	wp.RegisterHandler("retry", func(context.Context, *domain.Job, func(int, string)) error {
		called <- struct{}{}
		return errors.New("retry")
	})
	job, _ := wp.Enqueue("retry", nil, 2)
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	if err := wp.CancelJob(job.ID); err != nil {
		t.Fatal(err)
	}
	wp.Stop()
	select {
	case <-called:
		t.Fatal("handler retried after cancellation")
	default:
	}
	if _, err := wp.Enqueue("retry", nil, 0); err == nil {
		t.Fatal("enqueue after stop succeeded")
	}
}

func TestQueueFullSnapshotsAndPendingCancellation(t *testing.T) {
	wp := testPool(t, 0)
	job, _ := wp.Enqueue("test", map[string]interface{}{"name": "original"}, 0)
	job.Payload["name"] = "changed"
	job.Status = domain.JobStatusFailed
	stored, _ := wp.GetJob(job.ID)
	if stored.Status != domain.JobStatusPending || stored.Payload["name"] != "original" {
		t.Fatal("snapshot mutated stored job")
	}
	if err := wp.CancelJob(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := wp.Enqueue("test", nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := wp.Enqueue("test", nil, 0); err == nil {
		t.Fatal("full queue accepted job")
	}
	wp.Stop()
}
