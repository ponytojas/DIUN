package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// Scheduler manages periodic tasks for Docker image checking
type Scheduler struct {
	cron   *cron.Cron
	logger *logrus.Logger
	tasks  map[string]*Task
	mu     sync.RWMutex
}

// Task represents a scheduled task
type Task struct {
	ID          string
	Name        string
	Schedule    string
	Handler     TaskHandler
	LastRun     time.Time
	NextRun     time.Time
	RunCount    int64
	ErrorCount  int64
	IsRunning   bool
	cronEntryID cron.EntryID
	mu          sync.RWMutex
}

// TaskHandler is the function signature for task handlers
type TaskHandler func(ctx context.Context) error

// TaskStats contains statistics about a task
type TaskStats struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Schedule   string    `json:"schedule"`
	LastRun    time.Time `json:"last_run"`
	NextRun    time.Time `json:"next_run"`
	RunCount   int64     `json:"run_count"`
	ErrorCount int64     `json:"error_count"`
	IsRunning  bool      `json:"is_running"`
}

// NewScheduler creates a new scheduler instance
func NewScheduler(logger *logrus.Logger) *Scheduler {
	// Create cron with second precision and logging
	c := cron.New(
		cron.WithLocation(time.UTC),
		cron.WithLogger(cron.VerbosePrintfLogger(logger)),
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		),
	)

	return &Scheduler{
		cron:   c,
		logger: logger,
		tasks:  make(map[string]*Task),
	}
}

// AddTask adds a new task to the scheduler
func (s *Scheduler) AddTask(id, name, schedule string, handler TaskHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if task already exists
	if _, exists := s.tasks[id]; exists {
		return fmt.Errorf("task with ID %s already exists", id)
	}

	// Validate schedule
	if _, err := cron.ParseStandard(schedule); err != nil {
		return fmt.Errorf("invalid schedule format: %w", err)
	}

	// Create task
	task := &Task{
		ID:       id,
		Name:     name,
		Schedule: schedule,
		Handler:  handler,
	}

	// Wrap handler with logging and error handling
	wrappedHandler := s.wrapTaskHandler(task)

	// Add to cron
	entryID, err := s.cron.AddFunc(schedule, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to add task to cron: %w", err)
	}

	task.cronEntryID = entryID
	s.tasks[id] = task

	s.logger.WithFields(logrus.Fields{
		"task_id":   id,
		"task_name": name,
		"schedule":  schedule,
	}).Info("Added scheduled task")

	return nil
}

// RemoveTask removes a task from the scheduler
func (s *Scheduler) RemoveTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task with ID %s not found", id)
	}

	// Remove from cron
	s.cron.Remove(task.cronEntryID)

	// Remove from tasks map
	delete(s.tasks, id)

	s.logger.WithFields(logrus.Fields{
		"task_id":   id,
		"task_name": task.Name,
	}).Info("Removed scheduled task")

	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.cron.Start()
	s.logger.Info("Scheduler started")
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("Scheduler stopped")
}

// RunTask runs a task immediately (outside of its schedule)
func (s *Scheduler) RunTask(ctx context.Context, id string) error {
	s.mu.RLock()
	task, exists := s.tasks[id]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("task with ID %s not found", id)
	}

	task.mu.Lock()
	if task.IsRunning {
		task.mu.Unlock()
		return fmt.Errorf("task %s is already running", id)
	}
	task.IsRunning = true
	task.mu.Unlock()

	defer func() {
		task.mu.Lock()
		task.IsRunning = false
		task.mu.Unlock()
	}()

	s.logger.WithFields(logrus.Fields{
		"task_id":   id,
		"task_name": task.Name,
	}).Info("Running task manually")

	startTime := time.Now()
	err := task.Handler(ctx)
	duration := time.Since(startTime)

	task.mu.Lock()
	task.LastRun = startTime
	task.RunCount++
	if err != nil {
		task.ErrorCount++
	}
	task.mu.Unlock()

	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"task_id":   id,
			"task_name": task.Name,
			"duration":  duration,
		}).Error("Task execution failed")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"task_id":   id,
		"task_name": task.Name,
		"duration":  duration,
	}).Info("Task executed successfully")

	return nil
}

// GetTaskStats returns statistics for all tasks
func (s *Scheduler) GetTaskStats() []TaskStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make([]TaskStats, 0, len(s.tasks))

	for _, task := range s.tasks {
		task.mu.RLock()

		// Get next run time from cron
		entry := s.cron.Entry(task.cronEntryID)
		nextRun := entry.Next

		taskStats := TaskStats{
			ID:         task.ID,
			Name:       task.Name,
			Schedule:   task.Schedule,
			LastRun:    task.LastRun,
			NextRun:    nextRun,
			RunCount:   task.RunCount,
			ErrorCount: task.ErrorCount,
			IsRunning:  task.IsRunning,
		}
		task.mu.RUnlock()

		stats = append(stats, taskStats)
	}

	return stats
}

// GetTask returns information about a specific task
func (s *Scheduler) GetTask(id string) (*TaskStats, error) {
	s.mu.RLock()
	task, exists := s.tasks[id]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("task with ID %s not found", id)
	}

	task.mu.RLock()
	defer task.mu.RUnlock()

	// Get next run time from cron
	entry := s.cron.Entry(task.cronEntryID)
	nextRun := entry.Next

	return &TaskStats{
		ID:         task.ID,
		Name:       task.Name,
		Schedule:   task.Schedule,
		LastRun:    task.LastRun,
		NextRun:    nextRun,
		RunCount:   task.RunCount,
		ErrorCount: task.ErrorCount,
		IsRunning:  task.IsRunning,
	}, nil
}

// ListTasks returns a list of all task IDs
func (s *Scheduler) ListTasks() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.tasks))
	for id := range s.tasks {
		ids = append(ids, id)
	}
	return ids
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	// Check if any entries exist and cron is started
	entries := s.cron.Entries()
	return len(entries) > 0
}

// wrapTaskHandler wraps a task handler with logging and metrics
func (s *Scheduler) wrapTaskHandler(task *Task) func() {
	return func() {
		task.mu.Lock()
		if task.IsRunning {
			task.mu.Unlock()
			s.logger.WithField("task_id", task.ID).Warn("Task is already running, skipping")
			return
		}
		task.IsRunning = true
		task.mu.Unlock()

		defer func() {
			task.mu.Lock()
			task.IsRunning = false
			task.mu.Unlock()
		}()

		startTime := time.Now()

		s.logger.WithFields(logrus.Fields{
			"task_id":   task.ID,
			"task_name": task.Name,
		}).Debug("Starting scheduled task")

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Execute task
		err := task.Handler(ctx)
		duration := time.Since(startTime)

		// Update task statistics
		task.mu.Lock()
		task.LastRun = startTime
		task.RunCount++
		if err != nil {
			task.ErrorCount++
		}
		task.mu.Unlock()

		// Log result
		logFields := logrus.Fields{
			"task_id":   task.ID,
			"task_name": task.Name,
			"duration":  duration,
			"run_count": task.RunCount,
		}

		if err != nil {
			s.logger.WithError(err).WithFields(logFields).Error("Scheduled task failed")
		} else {
			s.logger.WithFields(logFields).Info("Scheduled task completed successfully")
		}
	}
}

// UpdateTaskSchedule updates the schedule for an existing task
func (s *Scheduler) UpdateTaskSchedule(id, newSchedule string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task with ID %s not found", id)
	}

	// Validate new schedule
	if _, err := cron.ParseStandard(newSchedule); err != nil {
		return fmt.Errorf("invalid schedule format: %w", err)
	}

	// Remove old cron entry
	s.cron.Remove(task.cronEntryID)

	// Create new cron entry
	wrappedHandler := s.wrapTaskHandler(task)
	entryID, err := s.cron.AddFunc(newSchedule, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to update task schedule: %w", err)
	}

	// Update task
	task.Schedule = newSchedule
	task.cronEntryID = entryID

	s.logger.WithFields(logrus.Fields{
		"task_id":      id,
		"task_name":    task.Name,
		"new_schedule": newSchedule,
	}).Info("Updated task schedule")

	return nil
}

// Health checks the health of the scheduler
func (s *Scheduler) Health() error {
	entries := s.cron.Entries()
	if len(entries) == 0 {
		return fmt.Errorf("no tasks scheduled")
	}

	// Check if any tasks have been failing consistently
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, task := range s.tasks {
		task.mu.RLock()
		if task.RunCount > 0 && task.ErrorCount == task.RunCount {
			task.mu.RUnlock()
			return fmt.Errorf("task %s is consistently failing", task.ID)
		}
		task.mu.RUnlock()
	}

	return nil
}
