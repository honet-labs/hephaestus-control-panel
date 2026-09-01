# Background Jobs, Worker Pool & Schedulers

Hephaestus implements a native, high-performance Go worker pool and cron scheduler, eliminating the overhead and operational complexity of Redis/RabbitMQ.

## 1. Architecture

```
                 [HTTP Request / Cron Trigger]
                               │
                               ▼
                   wp.Enqueue("job_type", payload)
                               │
                               ▼
                   ┌───────────────────────┐
                   │   Buffered Job Queue  │
                   │   (chan *domain.Job)  │
                   └───────────┬───────────┘
                               │
          ┌────────────────────┼────────────────────┐
          ▼                    ▼                    ▼
     [Worker 1]           [Worker 2]           [Worker N]
     (goroutine)          (goroutine)          (goroutine)
          │                    │                    │
          └────────────────────┼────────────────────┘
                               │
                               ▼
                   [Job Handler Execution]
                               │
                   ├── updateProgress(int, string)
                   └── context.WithCancel support
```

---

## 2. Job Lifecycle States
A job transitions through the following lifecycle:

1. **`pending`**: The job has been enqueued into the worker channel.
2. **`running`**: A worker goroutine has claimed the job and is actively executing its handler.
3. **`completed`**: Execution finished successfully (`progress = 100`).
4. **`failed`**: Execution returned an error after exhausting `maxRetries`.
5. **`cancelled`**: Job cancellation was requested and intercepted via `ctx.Done()`.

---

## 3. Registering New Job Handlers
To add a new background task handler, register it with the worker pool during service initialization:

```go
package myservice

import (
    "context"
    "go-hephaestus/internal/core/domain"
    "go-hephaestus/internal/queue"
)

type MyService struct{}

func NewMyService() *MyService {
    s := &MyService{}
    
    // Register handler name
    wp := queue.GetWorkerPool()
    wp.RegisterHandler("my_custom_task", s.HandleCustomTask)
    
    return s
}

func (s *MyService) HandleCustomTask(
    ctx context.Context, 
    job *domain.Job, 
    updateProgress func(progress int, message string),
) error {
    updateProgress(10, "Starting custom task...")
    
    select {
    case <-ctx.Done():
        return ctx.Err() // Handle cancellation
    default:
        // Perform workload...
    }
    
    updateProgress(100, "Finished custom task.")
    return nil
}
```

---

## 4. Cron Scheduling
Cron schedules are managed by `internal/scheduler/cron_scheduler.go`:
- **Database Backups**: Evaluates active backup profiles with user-defined cron expressions (e.g. `0 2 * * *` for daily at 2:00 AM).
- **ICMP Ping Sweep**: Automatically enqueues `icmp_ping_cycle` every 60 seconds.
- **Session Cleanup**: Removes expired tokens every 6 hours.
