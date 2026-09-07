package model

import (
	"time"

	"github.com/google/uuid"
)

// Todo mirrors the todos table and the Spring Boot TodoResponse shape.
// JSON field names match the existing REST contract (camelCase) so the
// frontend needs no changes.
type Todo struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CreateTodoRequest is the POST /api/todos body.
type CreateTodoRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
}

// UpdateTodoRequest is the PUT /api/todos/{id} body.
type UpdateTodoRequest struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Completed   bool    `json:"completed"`
}

// UpdateCompletionRequest is the PATCH /api/todos/{id}/complete body.
// Pointer so we can distinguish "missing" from "false" for validation.
type UpdateCompletionRequest struct {
	Completed *bool `json:"completed"`
}

// ApiError matches the Spring Boot error response shape exactly.
type ApiError struct {
	Timestamp   time.Time         `json:"timestamp"`
	Status      int               `json:"status"`
	Error       string            `json:"error"`
	Message     string            `json:"message"`
	FieldErrors map[string]string `json:"fieldErrors,omitempty"`
}
