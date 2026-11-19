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
	stopped     bool
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
	return InitWorkerPool(10)
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
	defer cancelJob()
	wp.jobsMu.Lock()
	if wp.stopped || job.Status == domain.JobStatusCancelled {
		wp.jobsMu.Unlock()
		return
	}
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
retryLoop:
	for attempt := 0; attempt <= job.MaxRetries; attempt++ {
		if jobCtx.Err() != nil {
			err = jobCtx.Err()
			break
		}
		if attempt > 0 {
			wp.jobsMu.Lock()
			job.Retries = attempt
			wp.jobsMu.Unlock()
			logger.Warn("Queue", fmt.Sprintf("[Worker %d] Retrying job %s (attempt %d/%d)", workerID, job.ID, attempt, job.MaxRetries))
			select {
			case <-jobCtx.Done():
				err = jobCtx.Err()
				break retryLoop
			case <-time.After(time.Duration(attempt*3) * time.Second):
			}
		}

		wp.jobsMu.RLock()
		handlerJob := snapshotJob(job)
		wp.jobsMu.RUnlock()
		err = runHandler(handler, jobCtx, handlerJob, updateProgress)
		if err == nil {
			break
		}
	}

	wp.jobsMu.Lock()
	if jobCtx.Err() != nil {
		err = jobCtx.Err()
	}
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
	if maxRetries < 0 {
		return nil, fmt.Errorf("max retries must not be negative")
	}
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
	job = snapshotJob(job)

	wp.jobsMu.Lock()
	defer wp.jobsMu.Unlock()
	if wp.stopped {
		return nil, fmt.Errorf("worker pool is stopped")
	}
	wp.jobs[job.ID] = job
	// Clean old jobs if store exceeds maximum capacity
	if len(wp.jobs) > wp.maxJobStore {
		var oldestID string
		var oldestTime time.Time
		for id, j := range wp.jobs {
			if j.Status == domain.JobStatusRunning || j.Status == domain.JobStatusPending {
				continue
			}
			if oldestTime.IsZero() || j.CreatedAt.Before(oldestTime) {
				oldestTime = j.CreatedAt
				oldestID = id
			}
		}
		if oldestID != "" && oldestID != job.ID {
			delete(wp.jobs, oldestID)
		}
	}
	select {
	case wp.jobQueue <- job:
		return snapshotJob(job), nil
	default:
		job.Status = domain.JobStatusFailed
		job.Error = "Job queue is currently full"
		job.Message = job.Error
		return nil, fmt.Errorf("job queue is full")
	}
}

func (wp *WorkerPool) GetJob(id string) (*domain.Job, bool) {
	wp.jobsMu.RLock()
	defer wp.jobsMu.RUnlock()
	j, exists := wp.jobs[id]
	if !exists {
		return nil, false
	}
	return snapshotJob(j), true
}

func (wp *WorkerPool) ListJobs() []*domain.Job {
	wp.jobsMu.RLock()
	defer wp.jobsMu.RUnlock()
	list := make([]*domain.Job, 0, len(wp.jobs))
	for _, j := range wp.jobs {
		list = append(list, snapshotJob(j))
	}
	return list
}

func (wp *WorkerPool) CancelJob(id string) error {
	wp.jobsMu.Lock()
	defer wp.jobsMu.Unlock()
	if job, ok := wp.jobs[id]; ok && job.Status == domain.JobStatusPending {
		job.Status = domain.JobStatusCancelled
		job.Message = "Job cancelled"
		return nil
	}
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
		if j.Status == domain.JobStatusCancelled {
			return
		}
		j.Status = domain.JobStatusFailed
		j.Error = reason
		j.Message = reason
	}
}

func (wp *WorkerPool) Stop() {
	wp.jobsMu.Lock()
	wp.stopped = true
	for _, job := range wp.jobs {
		if job.Status == domain.JobStatusPending {
			job.Status = domain.JobStatusCancelled
			job.Message = "Worker pool stopped"
		}
	}
	wp.cancel()
	wp.jobsMu.Unlock()
	wp.wg.Wait()
	logger.Info("Queue", "Background worker pool stopped.")
}

func snapshotJob(job *domain.Job) *domain.Job {
	copy := *job
	copy.Payload = make(map[string]interface{}, len(job.Payload))
	for key, value := range job.Payload {
		copy.Payload[key] = snapshotValue(value)
	}
	if job.StartedAt != nil {
		value := *job.StartedAt
		copy.StartedAt = &value
	}
	if job.CompletedAt != nil {
		value := *job.CompletedAt
		copy.CompletedAt = &value
	}
	return &copy
}

// Job payloads are JSON-shaped; detach nested mutable maps and arrays too.
func snapshotValue(value interface{}) interface{} {
	switch value := value.(type) {
	case map[string]interface{}:
		copy := make(map[string]interface{}, len(value))
		for key, item := range value {
			copy[key] = snapshotValue(item)
		}
		return copy
	case []interface{}:
		copy := make([]interface{}, len(value))
		for i, item := range value {
			copy[i] = snapshotValue(item)
		}
		return copy
	case []string:
		return append([]string(nil), value...)
	case map[string]string:
		copy := make(map[string]string, len(value))
		for key, item := range value {
			copy[key] = item
		}
		return copy
	default:
		return value
	}
}

func runHandler(handler JobHandlerFunc, ctx context.Context, job *domain.Job, progress func(int, string)) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("job handler panicked: %v", recovered)
		}
	}()
	return handler(ctx, job, progress)
}

type WorkerPoolStats struct {
	TotalWorkers  int            `json:"totalWorkers"`
	RunningJobs   int            `json:"runningJobs"`
	QueuedJobs    int            `json:"queuedJobs"`
	RunningByType map[string]int `json:"runningByType"`
	QueuedByType  map[string]int `json:"queuedByType"`
}

func (wp *WorkerPool) GetStats() WorkerPoolStats {
	wp.jobsMu.RLock()
	defer wp.jobsMu.RUnlock()

	stats := WorkerPoolStats{
		TotalWorkers:  wp.numWorkers,
		QueuedJobs:    len(wp.jobQueue),
		RunningByType: make(map[string]int),
		QueuedByType:  make(map[string]int),
	}

	for _, j := range wp.jobs {
		switch j.Status {
		case domain.JobStatusRunning:
			stats.RunningJobs++
			stats.RunningByType[j.Type]++
		case domain.JobStatusPending:
			stats.QueuedByType[j.Type]++
		}
	}
	return stats
}
