package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"

	"github.com/KemenyStudio/task-manager/internal/db"
	"github.com/KemenyStudio/task-manager/internal/middleware"
	"github.com/KemenyStudio/task-manager/internal/model"
)

var jwtSecret []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default-secret-change-in-production"
	}
	jwtSecret = []byte(secret)
}

// ListTasks returns all tasks, optionally filtered by status and with assignee data.
func ListTasks(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	include := r.URL.Query().Get("include")

	query := "SELECT id, title, description, status, priority, category, summary, creator_id, assignee_id, due_date, estimated_hours, actual_hours, created_at, updated_at FROM tasks"
	args := []interface{}{}

	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.Pool.Query(r.Context(), query, args...)
	if err != nil {
		http.Error(w, `{"error": "failed to query tasks"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := []model.Task{}
	for rows.Next() {
		var t model.Task
		err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.Category, &t.Summary, &t.CreatorID, &t.AssigneeID,
			&t.DueDate, &t.EstimatedHours, &t.ActualHours,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			log.Printf("error scanning task: %v", err)
			continue
		}
		tasks = append(tasks, t)
	}

	// Load assignee for each task
	if include == "assignee" {
		for i, t := range tasks {
			if t.AssigneeID != nil {
				var user model.User
				err := db.Pool.QueryRow(r.Context(),
					"SELECT id, email, name, role, avatar_url, created_at, updated_at FROM users WHERE id = $1",
					*t.AssigneeID,
				).Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
				if err == nil {
					tasks[i].Assignee = &user
				}
			}
		}
	}

	// Load tags for each task
	for i, t := range tasks {
		rows, err := db.Pool.Query(r.Context(),
			`SELECT t.id, t.name, t.color, t.created_at
			 FROM tags t
			 INNER JOIN task_tags tt ON t.id = tt.tag_id
			 WHERE tt.task_id = $1`, t.ID)
		if err != nil {
			continue
		}
		var tags []model.Tag
		for rows.Next() {
			var tag model.Tag
			_ = rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt)
			tags = append(tags, tag)
		}
		rows.Close()
		tasks[i].Tags = tags
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// GetTask returns a single task by ID with full details.
func GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	var t model.Task
	err := db.Pool.QueryRow(r.Context(),
		`SELECT id, title, description, status, priority, category, summary,
		        creator_id, assignee_id, due_date, estimated_hours, actual_hours,
		        created_at, updated_at
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.Category, &t.Summary, &t.CreatorID, &t.AssigneeID,
		&t.DueDate, &t.EstimatedHours, &t.ActualHours,
		&t.CreatedAt, &t.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		http.Error(w, `{"error": "task not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error": "failed to get task"}`, http.StatusInternalServerError)
		return
	}

	// Load creator
	var creator model.User
	_ = db.Pool.QueryRow(r.Context(),
		"SELECT id, email, name, role, avatar_url, created_at, updated_at FROM users WHERE id = $1",
		t.CreatorID,
	).Scan(&creator.ID, &creator.Email, &creator.Name, &creator.Role, &creator.AvatarURL, &creator.CreatedAt, &creator.UpdatedAt)
	t.Creator = &creator

	// Load assignee
	if t.AssigneeID != nil {
		var assignee model.User
		_ = db.Pool.QueryRow(r.Context(),
			"SELECT id, email, name, role, avatar_url, created_at, updated_at FROM users WHERE id = $1",
			*t.AssigneeID,
		).Scan(&assignee.ID, &assignee.Email, &assignee.Name, &assignee.Role, &assignee.AvatarURL, &assignee.CreatedAt, &assignee.UpdatedAt)
		t.Assignee = &assignee
	}

	// Load tags
	tagRows, err := db.Pool.Query(r.Context(),
		`SELECT t.id, t.name, t.color, t.created_at
		 FROM tags t
		 INNER JOIN task_tags tt ON t.id = tt.tag_id
		 WHERE tt.task_id = $1`, t.ID)
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var tag model.Tag
			_ = tagRows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt)
			t.Tags = append(t.Tags, tag)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

// CreateTask creates a new task.
func CreateTask(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req model.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, `{"error": "title is required"}`, http.StatusBadRequest)
		return
	}

	if len(req.Title) > 500 {
		http.Error(w, `{"error": "title too long"}`, http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{"todo": true, "in_progress": true, "review": true, "done": true}
	if req.Status == "" {
		req.Status = "todo"
	}
	if !validStatuses[req.Status] {
		http.Error(w, `{"error": "invalid status"}`, http.StatusBadRequest)
		return
	}

	validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}
	if req.Priority == "" {
		req.Priority = "medium"
	}
	if !validPriorities[req.Priority] {
		http.Error(w, `{"error": "invalid priority"}`, http.StatusBadRequest)
		return
	}

	// Validate assignee exists if provided
	if req.AssigneeID != nil && *req.AssigneeID != "" {
		var exists bool
		err := db.Pool.QueryRow(r.Context(),
			"SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", *req.AssigneeID,
		).Scan(&exists)
		if err != nil || !exists {
			http.Error(w, `{"error": "assignee not found"}`, http.StatusBadRequest)
			return
		}
	}

	// Parse due date if provided
	var dueDate *time.Time
	if req.DueDate != nil && *req.DueDate != "" {
		parsed, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			http.Error(w, `{"error": "invalid due_date format, use RFC3339"}`, http.StatusBadRequest)
			return
		}
		dueDate = &parsed
	}

	var task model.Task
	err := db.Pool.QueryRow(r.Context(),
		`INSERT INTO tasks (title, description, status, priority, creator_id, assignee_id, due_date, estimated_hours)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, title, description, status, priority, category, summary, creator_id, assignee_id, due_date, estimated_hours, actual_hours, created_at, updated_at`,
		req.Title, req.Description, req.Status, req.Priority, userID, req.AssigneeID, dueDate, req.EstimatedHours,
	).Scan(
		&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority,
		&task.Category, &task.Summary, &task.CreatorID, &task.AssigneeID,
		&task.DueDate, &task.EstimatedHours, &task.ActualHours,
		&task.CreatedAt, &task.UpdatedAt,
	)

	if err != nil {
		log.Printf("error creating task: %v", err)
		http.Error(w, `{"error": "failed to create task"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// UpdateTask updates an existing task.
func UpdateTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	var req model.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Fetch current task state
	var existing model.Task
	err := db.Pool.QueryRow(r.Context(),
		`SELECT id, title, description, status, priority, creator_id, assignee_id,
		        due_date, estimated_hours, actual_hours
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(
		&existing.ID, &existing.Title, &existing.Description, &existing.Status,
		&existing.Priority, &existing.CreatorID, &existing.AssigneeID,
		&existing.DueDate, &existing.EstimatedHours, &existing.ActualHours,
	)

	if err == pgx.ErrNoRows {
		http.Error(w, `{"error": "task not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error": "failed to get task"}`, http.StatusInternalServerError)
		return
	}

	// Build update fields
	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.Status != nil {
		validStatuses := map[string]bool{"todo": true, "in_progress": true, "review": true, "done": true}
		if !validStatuses[*req.Status] {
			http.Error(w, `{"error": "invalid status"}`, http.StatusBadRequest)
			return
		}
		existing.Status = *req.Status
	}
	if req.Priority != nil {
		validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}
		if !validPriorities[*req.Priority] {
			http.Error(w, `{"error": "invalid priority"}`, http.StatusBadRequest)
			return
		}
		existing.Priority = *req.Priority
	}
	if req.AssigneeID != nil {
		existing.AssigneeID = req.AssigneeID
	}
	if req.EstimatedHours != nil {
		existing.EstimatedHours = req.EstimatedHours
	}
	if req.ActualHours != nil {
		existing.ActualHours = req.ActualHours
	}

	// Start transaction
	tx, err := db.Pool.Begin(r.Context())
	if err != nil {
		http.Error(w, `{"error": "failed to start transaction"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	// Update task
	_, err = tx.Exec(r.Context(),
		`UPDATE tasks SET title=$1, description=$2, status=$3, priority=$4,
		 assignee_id=$5, estimated_hours=$6, actual_hours=$7, updated_at=NOW()
		 WHERE id=$8`,
		existing.Title, existing.Description, existing.Status, existing.Priority,
		existing.AssigneeID, existing.EstimatedHours, existing.ActualHours,
		taskID,
	)
	if err != nil {
		http.Error(w, `{"error": "failed to update task"}`, http.StatusInternalServerError)
		return
	}

	// Record edit history if status changed
	userID := middleware.GetUserID(r)
	if req.Status != nil {
		_, err = tx.Exec(r.Context(),
			`INSERT INTO edit_history (task_id, user_id, field_name, old_value, new_value)
			 VALUES ($1, $2, 'status', $3, $4)`,
			taskID, userID, existing.Status, *req.Status,
		)
		if err != nil {
			http.Error(w, `{"error": "failed to record edit history"}`, http.StatusInternalServerError)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, `{"error": "failed to commit transaction"}`, http.StatusInternalServerError)
		return
	}

	// Return updated task
	var updated model.Task
	err = db.Pool.QueryRow(r.Context(),
		`SELECT id, title, description, status, priority, category, summary,
		        creator_id, assignee_id, due_date, estimated_hours, actual_hours,
		        created_at, updated_at
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(
		&updated.ID, &updated.Title, &updated.Description, &updated.Status,
		&updated.Priority, &updated.Category, &updated.Summary,
		&updated.CreatorID, &updated.AssigneeID,
		&updated.DueDate, &updated.EstimatedHours, &updated.ActualHours,
		&updated.CreatedAt, &updated.UpdatedAt,
	)

	if err != nil {
		http.Error(w, `{"error": "failed to retrieve updated task"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// DeleteTask deletes a task by ID.
func DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	result, err := db.Pool.Exec(r.Context(),
		"DELETE FROM tasks WHERE id = $1", taskID,
	)

	if err != nil {
		http.Error(w, `{"error": "failed to delete task"}`, http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, `{"error": "task not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetTaskHistory returns the edit history for a task.
func GetTaskHistory(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	rows, err := db.Pool.Query(r.Context(),
		`SELECT id, task_id, user_id, field_name, old_value, new_value, edited_at
		 FROM edit_history WHERE task_id = $1 ORDER BY edited_at DESC`, taskID)
	if err != nil {
		http.Error(w, `{"error": "failed to get history"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	history := []model.EditHistory{}
	for rows.Next() {
		var h model.EditHistory
		_ = rows.Scan(&h.ID, &h.TaskID, &h.UserID, &h.FieldName, &h.OldValue, &h.NewValue, &h.EditedAt)
		history = append(history, h)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// SearchTasks searches tasks by title or description.
func SearchTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, `{"error": "query parameter q is required"}`, http.StatusBadRequest)
		return
	}

	searchTerm := "%" + strings.ToLower(q) + "%"

	rows, err := db.Pool.Query(r.Context(),
		`SELECT id, title, description, status, priority, category, summary,
		        creator_id, assignee_id, due_date, estimated_hours, actual_hours,
		        created_at, updated_at
		 FROM tasks
		 WHERE LOWER(title) LIKE $1 OR LOWER(COALESCE(description, '')) LIKE $1
		 ORDER BY created_at DESC`,
		searchTerm,
	)
	if err != nil {
		http.Error(w, `{"error": "search failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := []model.Task{}
	for rows.Next() {
		var t model.Task
		err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.Category, &t.Summary, &t.CreatorID, &t.AssigneeID,
			&t.DueDate, &t.EstimatedHours, &t.ActualHours,
			&t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			continue
		}
		tasks = append(tasks, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// GetDashboardStats returns summary statistics for the dashboard.
func GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	type Stats struct {
		TotalTasks   int            `json:"total_tasks"`
		ByStatus     map[string]int `json:"by_status"`
		ByPriority   map[string]int `json:"by_priority"`
		OverdueTasks int            `json:"overdue_tasks"`
	}

	var stats Stats
	stats.ByStatus = make(map[string]int)
	stats.ByPriority = make(map[string]int)

	// Total
	err := db.Pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM tasks").Scan(&stats.TotalTasks)
	if err != nil {
		http.Error(w, `{"error": "failed to get stats"}`, http.StatusInternalServerError)
		return
	}

	// By status
	rows, err := db.Pool.Query(r.Context(), "SELECT status, COUNT(*) FROM tasks GROUP BY status")
	if err != nil {
		http.Error(w, `{"error": "failed to get stats"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		_ = rows.Scan(&status, &count)
		stats.ByStatus[status] = count
	}

	// By priority
	rows2, err := db.Pool.Query(r.Context(), "SELECT priority, COUNT(*) FROM tasks GROUP BY priority")
	if err != nil {
		http.Error(w, `{"error": "failed to get stats"}`, http.StatusInternalServerError)
		return
	}
	defer rows2.Close()
	for rows2.Next() {
		var priority string
		var count int
		_ = rows2.Scan(&priority, &count)
		stats.ByPriority[priority] = count
	}

	// Overdue
	_ = db.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM tasks WHERE due_date < NOW() AND status != 'done'",
	).Scan(&stats.OverdueTasks)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// LoginHandler handles user authentication and returns a JWT token.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	var user model.User
	err := db.Pool.QueryRow(r.Context(),
		"SELECT id, email, name, password_hash, role FROM users WHERE email = $1",
		req.Email,
	).Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.Role)

	if err != nil {
		http.Error(w, `{"error": "invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// NOTE: In a real app, we'd use bcrypt.CompareHashAndPassword here.
	// For the assessment, we accept any password for seeded users.
	// This is intentional to simplify testing.
	_ = user.PasswordHash

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   user.ID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		Issuer:    "task-manager",
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, `{"error": "failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": tokenString,
		"user": map[string]string{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
			"role":  user.Role,
		},
	})
}
