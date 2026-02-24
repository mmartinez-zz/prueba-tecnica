package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KemenyStudio/task-manager/internal/llm"
	"github.com/KemenyStudio/task-manager/internal/model"
)

type TaskService struct {
	DB  *pgxpool.Pool
	LLM llm.LLMClient
}

func (s *TaskService) ClassifyTask(ctx context.Context, taskID string) (*model.Task, error) {
	// Get task
	var task model.Task
	err := s.DB.QueryRow(ctx,
		`SELECT id, title, description, status, priority, category, summary,
		        creator_id, assignee_id, due_date, estimated_hours, actual_hours,
		        created_at, updated_at
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(
		&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority,
		&task.Category, &task.Summary, &task.CreatorID, &task.AssigneeID,
		&task.DueDate, &task.EstimatedHours, &task.ActualHours,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("task not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	var description string
	if task.Description != nil {
		description = *task.Description
	}

	// Call LLM
	classification, err := s.LLM.ClassifyTask(ctx, task.Title, description)
	if err != nil {
		return nil, fmt.Errorf("failed to classify task: %w", err)
	}

	// Start transaction
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert tags
	for _, tagName := range classification.Tags {
		_, err = tx.Exec(ctx, `INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`, tagName)
		if err != nil {
			return nil, fmt.Errorf("failed to insert tag: %w", err)
		}

		var tagID string
		err = tx.QueryRow(ctx, `SELECT id FROM tags WHERE name = $1`, tagName).Scan(&tagID)
		if err != nil {
			return nil, fmt.Errorf("failed to get tag ID: %w", err)
		}

		_, err = tx.Exec(ctx, `INSERT INTO task_tags (task_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, taskID, tagID)
		if err != nil {
			return nil, fmt.Errorf("failed to insert task tag: %w", err)
		}
	}

	// Update task
	_, err = tx.Exec(ctx,
		`UPDATE tasks SET priority = $1, category = $2, summary = $3 WHERE id = $4`,
		classification.Priority, classification.Category, classification.Summary, taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Load full task
	return s.loadFullTask(ctx, taskID)
}

func (s *TaskService) loadFullTask(ctx context.Context, taskID string) (*model.Task, error) {
	var task model.Task
	err := s.DB.QueryRow(ctx,
		`SELECT id, title, description, status, priority, category, summary,
		        creator_id, assignee_id, due_date, estimated_hours, actual_hours,
		        created_at, updated_at
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(
		&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority,
		&task.Category, &task.Summary, &task.CreatorID, &task.AssigneeID,
		&task.DueDate, &task.EstimatedHours, &task.ActualHours,
		&task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load task: %w", err)
	}

	// Load creator
	var creator model.User
	err = s.DB.QueryRow(ctx,
		"SELECT id, email, name, role, avatar_url, created_at, updated_at FROM users WHERE id = $1",
		task.CreatorID,
	).Scan(&creator.ID, &creator.Email, &creator.Name, &creator.Role, &creator.AvatarURL, &creator.CreatedAt, &creator.UpdatedAt)
	if err == nil {
		task.Creator = &creator
	}

	// Load assignee
	if task.AssigneeID != nil {
		var assignee model.User
		err = s.DB.QueryRow(ctx,
			"SELECT id, email, name, role, avatar_url, created_at, updated_at FROM users WHERE id = $1",
			*task.AssigneeID,
		).Scan(&assignee.ID, &assignee.Email, &assignee.Name, &assignee.Role, &assignee.AvatarURL, &assignee.CreatedAt, &assignee.UpdatedAt)
		if err == nil {
			task.Assignee = &assignee
		}
	}

	// Load tags
	rows, err := s.DB.Query(ctx,
		`SELECT t.id, t.name, t.color, t.created_at
		 FROM tags t
		 INNER JOIN task_tags tt ON t.id = tt.tag_id
		 WHERE tt.task_id = $1`, task.ID)
	if err == nil {
		defer rows.Close()
		var tags []model.Tag
		for rows.Next() {
			var tag model.Tag
			err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt)
			if err == nil {
				tags = append(tags, tag)
			}
		}
		task.Tags = tags
	}

	return &task, nil
}