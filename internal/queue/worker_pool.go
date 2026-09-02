package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-hephaestus/internal/core/domain"
	"go-hephaestus/internal/logger"

	"github.com/google/uuid"
)

type JobHandlerFunc func(ctx context.Context, job *domain.Job, updateProgress func(progress int, msg string)) error

type WorkerPool struct {
	numWorkers  int
	jobQueue    chan *domain.Job
	handlers    map[string]JobHandlerFunc
	handlersMu  sync.RWMutex
	jobs        map[string]*domain.Job
	cancels     map[string]context.CancelFunc
	jobsMu      sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	maxJobStore int
}

var (
	defaultPool *WorkerPool
	poolOnce    sync.Once
)

// InitWorkerPool initializes the global worker pool with n workers
func InitWorkerPool(workers int) *WorkerPool {
	poolOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		defaultPool = &WorkerPool{
			numWorkers:  workers,
			jobQueue:    make(chan *domain.Job, 100),
			handlers:    make(map[string]JobHandlerFunc),
			jobs:        make(map[string]*domain.Job),
			cancels:     make(map[string]context.CancelFunc),
			ctx:         ctx,
			cancel:      cancel,
			maxJobStore: 200,
		}

		defaultPool.start()
		logger.Info("Queue", fmt.Sprintf("Background worker pool started with %d workers", workers))
	})
	return defaultPool
}

// GetWorkerPool returns the global worker pool
func GetWorkerPool() *WorkerPool {
	if defaultPool == nil {
		return InitWorkerPool(5)
	}
	return defaultPool
}

func (wp *WorkerPool) RegisterHandler(jobType string, handler JobHandlerFunc) {
	wp.handlersMu.Lock()
	defer wp.handlersMu.Unlock()
	wp.handlers[jobType] = handler
}

func (wp *WorkerPool) start() {
	for i := 1; i <= wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}
			wp.processJob(id, job)
		}
	}
}

func (wp *WorkerPool) processJob(workerID int, job *domain.Job) {
	wp.handlersMu.RLock()
	handler, exists := wp.handlers[job.Type]
	wp.handlersMu.RUnlock()

	if !exists {
		wp.updateJobFailed(job.ID, fmt.Sprintf("No handler registered for job type: %s", job.Type))
		return
	}

	jobCtx, cancelJob := context.WithCancel(wp.ctx)
	wp.jobsMu.Lock()
	now := time.Now()
	job.Status = domain.JobStatusRunning
	job.StartedAt = &now
	wp.cancels[job.ID] = cancelJob
	wp.jobsMu.Unlock()

	logger.Info("Queue", fmt.Sprintf("[Worker %d] Starting job %s (%s)", workerID, job.ID, job.Type))

	updateProgress := func(progress int, msg string) {
		wp.jobsMu.Lock()
		defer wp.jobsMu.Unlock()
		job.Progress = progress
		job.Message = msg
	}

	var err error
	for attempt := 0; attempt <= job.MaxRetries; attempt++ {
		if attempt > 0 {
			job.Retries = attempt
			logger.Warn("Queue", fmt.Sprintf("[Worker %d] Retrying job %s (attempt %d/%d)", workerID, job.ID, attempt, job.MaxRetries))
			select {
			case <-jobCtx.Done():
				err = jobCtx.Err()
				break
			case <-time.After(time.Duration(attempt*3) * time.Second):
			}
		}

		err = handler(jobCtx, job, updateProgress)
		if err == nil {
			break
		}
	}

	wp.jobsMu.Lock()
	delete(wp.cancels, job.ID)
	completedAt := time.Now()
	job.CompletedAt = &completedAt
	if err != nil {
		if jobCtx.Err() == context.Canceled {
			job.Status = domain.JobStatusCancelled
			job.Message = "Job cancelled by user"
		} else {
			job.Status = domain.JobStatusFailed
			job.Error = err.Error()
			job.Message = "Job execution failed"
		}
		logger.Error("Queue", fmt.Sprintf("[Worker %d] Job %s failed", workerID, job.ID), err)
	} else {
		job.Status = domain.JobStatusCompleted
		job.Progress = 100
		job.Message = "Job completed successfully"
		logger.Info("Queue", fmt.Sprintf("[Worker %d] Job %s completed successfully", workerID, job.ID))
	}
	wp.jobsMu.Unlock()
}

// Enqueue adds a job to the queue
func (wp *WorkerPool) Enqueue(jobType string, payload map[string]interface{}, maxRetries int) (*domain.Job, error) {
	job := &domain.Job{
		ID:         fmt.Sprintf("job-%s", uuid.New().String()[:8]),
		Type:       jobType,
		Status:     domain.JobStatusPending,
		Payload:    payload,
		Progress:   0,
		Message:    "Queued for execution",
		MaxRetries: maxRetries,
		CreatedAt:  time.Now(),
	}

	wp.jobsMu.Lock()
	wp.jobs[job.ID] = job
	// Clean old jobs if store exceeds maximum capacity
	if len(wp.jobs) > wp.maxJobStore {
		var oldestID string
		var oldestTime time.Time
		for id, j := range wp.jobs {
			if oldestTime.IsZero() || j.CreatedAt.Before(oldestTime) {
				oldestTime = j.CreatedAt
				oldestID = id
			}
		}
		if oldestID != "" && oldestID != job.ID {
			delete(wp.jobs, oldestID)
		}
	}
	wp.jobsMu.Unlock()

	select {
	case wp.jobQueue <- job:
		return job, nil
	default:
		wp.updateJobFailed(job.ID, "Job queue is currently full")
		return nil, fmt.Errorf("job queue is full")
	}
}

func (wp *WorkerPool) GetJob(id string) (*domain.Job, bool) {
	wp.jobsMu.RLock()
	defer wp.jobsMu.RUnlock()
	j, exists := wp.jobs[id]
	return j, exists
}

func (wp *WorkerPool) ListJobs() []*domain.Job {
	wp.jobsMu.RLock()
	defer wp.jobsMu.RUnlock()
	list := make([]*domain.Job, 0, len(wp.jobs))
	for _, j := range wp.jobs {
		list = append(list, j)
	}
	return list
}

func (wp *WorkerPool) CancelJob(id string) error {
	wp.jobsMu.Lock()
	defer wp.jobsMu.Unlock()
	if cancel, exists := wp.cancels[id]; exists {
		cancel()
		delete(wp.cancels, id)
		if j, ok := wp.jobs[id]; ok {
			j.Status = domain.JobStatusCancelled
			j.Message = "Job cancelled"
		}
		return nil
	}
	return fmt.Errorf("job %s is not currently running", id)
}

func (wp *WorkerPool) updateJobFailed(id, reason string) {
	wp.jobsMu.Lock()
	defer wp.jobsMu.Unlock()
	if j, ok := wp.jobs[id]; ok {
		j.Status = domain.JobStatusFailed
		j.Error = reason
		j.Message = reason
	}
}

func (wp *WorkerPool) Stop() {
	wp.cancel()
	close(wp.jobQueue)
	wp.wg.Wait()
	logger.Info("Queue", "Background worker pool stopped.")
}
