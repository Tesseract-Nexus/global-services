package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/models"
	"gorm.io/gorm"
)

// TaskRepository handles onboarding task operations
type TaskRepository struct {
	db *gorm.DB
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{
		db: db,
	}
}

// CreateTask creates a new onboarding task
func (r *TaskRepository) CreateTask(ctx context.Context, task *models.OnboardingTask) (*models.OnboardingTask, error) {
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		return nil, fmt.Errorf("failed to create onboarding task: %w", err)
	}

	return task, nil
}

// CreateTasksBatch creates multiple tasks in a batch
func (r *TaskRepository) CreateTasksBatch(ctx context.Context, tasks []models.OnboardingTask) ([]models.OnboardingTask, error) {
	// Generate IDs for tasks that don't have them
	for i := range tasks {
		if tasks[i].ID == uuid.Nil {
			tasks[i].ID = uuid.New()
		}
	}

	if err := r.db.WithContext(ctx).Create(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to create tasks batch: %w", err)
	}

	return tasks, nil
}

// GetTaskByID retrieves a task by ID
func (r *TaskRepository) GetTaskByID(ctx context.Context, id uuid.UUID) (*models.OnboardingTask, error) {
	var task models.OnboardingTask

	if err := r.db.WithContext(ctx).Preload("ExecutionLogs").First(&task, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("onboarding task not found")
		}
		return nil, fmt.Errorf("failed to get onboarding task: %w", err)
	}

	return &task, nil
}

// GetTasksBySession retrieves all tasks for a session
func (r *TaskRepository) GetTasksBySession(ctx context.Context, sessionID uuid.UUID) ([]models.OnboardingTask, error) {
	var tasks []models.OnboardingTask

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ?", sessionID).
		Order("order_index ASC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to get tasks by session: %w", err)
	}

	return tasks, nil
}

// GetTaskBySessionAndTaskID retrieves a task by session ID and task_id string
func (r *TaskRepository) GetTaskBySessionAndTaskID(ctx context.Context, sessionID uuid.UUID, taskID string) (*models.OnboardingTask, error) {
	var task models.OnboardingTask

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND task_id = ?", sessionID, taskID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("task %s not found for session", taskID)
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return &task, nil
}

// GetTasksBySessionAndStatus retrieves tasks by session and status
func (r *TaskRepository) GetTasksBySessionAndStatus(ctx context.Context, sessionID uuid.UUID, status string) ([]models.OnboardingTask, error) {
	var tasks []models.OnboardingTask

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND status = ?", sessionID, status).
		Order("order_index ASC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to get tasks by session and status: %w", err)
	}

	return tasks, nil
}

// UpdateTask updates a task in the database
func (r *TaskRepository) UpdateTask(ctx context.Context, task *models.OnboardingTask) error {
	if err := r.db.WithContext(ctx).Save(task).Error; err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	return nil
}

// GetNextPendingTask retrieves the next pending task for a session
func (r *TaskRepository) GetNextPendingTask(ctx context.Context, sessionID uuid.UUID) (*models.OnboardingTask, error) {
	var task models.OnboardingTask

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND status = ?",
		sessionID, "pending").Order("order_index ASC").First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no pending tasks found")
		}
		return nil, fmt.Errorf("failed to get next pending task: %w", err)
	}

	return &task, nil
}

// UpdateTaskStatus updates the status of a task
func (r *TaskRepository) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string, completionData map[string]interface{}) error {
	updates := map[string]interface{}{
		"status": status,
	}

	now := time.Now()
	switch status {
	case "in_progress":
		updates["started_at"] = &now
	case "completed":
		updates["completed_at"] = &now
		if completionData != nil {
			updates["completion_data"] = completionData
		}
	case "skipped":
		updates["skipped_at"] = &now
	}

	if err := r.db.WithContext(ctx).Model(&models.OnboardingTask{}).
		Where("id = ?", taskID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

// SkipTask marks a task as skipped with a reason
func (r *TaskRepository) SkipTask(ctx context.Context, taskID uuid.UUID, reason string) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&models.OnboardingTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":      "skipped",
			"skipped_at":  &now,
			"skip_reason": reason,
		}).Error; err != nil {
		return fmt.Errorf("failed to skip task: %w", err)
	}

	return nil
}

// GetTaskProgress retrieves task progress for a session
func (r *TaskRepository) GetTaskProgress(ctx context.Context, sessionID uuid.UUID) (map[string]int, error) {
	type StatusCount struct {
		Status string
		Count  int
	}

	var statusCounts []StatusCount
	if err := r.db.WithContext(ctx).Model(&models.OnboardingTask{}).
		Select("status, COUNT(*) as count").
		Where("onboarding_session_id = ?", sessionID).
		Group("status").Find(&statusCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get task progress: %w", err)
	}

	progress := map[string]int{
		"pending":     0,
		"in_progress": 0,
		"completed":   0,
		"skipped":     0,
		"failed":      0,
	}

	for _, sc := range statusCounts {
		progress[sc.Status] = sc.Count
	}

	return progress, nil
}

// GetFailedTasks retrieves all failed tasks for a session
func (r *TaskRepository) GetFailedTasks(ctx context.Context, sessionID uuid.UUID) ([]models.OnboardingTask, error) {
	var tasks []models.OnboardingTask

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND status = ?",
		sessionID, "failed").Order("order_index ASC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to get failed tasks: %w", err)
	}

	return tasks, nil
}

// GetRequiredIncompleteTasks retrieves required tasks that are not completed for a session
func (r *TaskRepository) GetRequiredIncompleteTasks(ctx context.Context, sessionID uuid.UUID) ([]models.OnboardingTask, error) {
	var tasks []models.OnboardingTask

	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ? AND is_required = ? AND status NOT IN (?)",
		sessionID, true, []string{"completed", "skipped"}).Order("order_index ASC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to get required incomplete tasks: %w", err)
	}

	return tasks, nil
}

// DeleteTasksBySession deletes all tasks for a session
func (r *TaskRepository) DeleteTasksBySession(ctx context.Context, sessionID uuid.UUID) error {
	if err := r.db.WithContext(ctx).Where("onboarding_session_id = ?", sessionID).
		Delete(&models.OnboardingTask{}).Error; err != nil {
		return fmt.Errorf("failed to delete tasks by session: %w", err)
	}

	return nil
}

// CreateTaskExecutionLog creates a task execution log entry
func (r *TaskRepository) CreateTaskExecutionLog(ctx context.Context, log *models.TaskExecutionLog) (*models.TaskExecutionLog, error) {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return nil, fmt.Errorf("failed to create task execution log: %w", err)
	}

	return log, nil
}

// GetTaskExecutionLogs retrieves execution logs for a task
func (r *TaskRepository) GetTaskExecutionLogs(ctx context.Context, taskID uuid.UUID) ([]models.TaskExecutionLog, error) {
	var logs []models.TaskExecutionLog

	if err := r.db.WithContext(ctx).Where("onboarding_task_id = ?", taskID).
		Order("created_at ASC").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get task execution logs: %w", err)
	}

	return logs, nil
}
