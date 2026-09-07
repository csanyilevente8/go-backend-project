package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csanyilevente8/go-backend-project/internal/model"
	"github.com/csanyilevente8/go-backend-project/internal/repository"
)

// TodoStore is the persistence behaviour the handlers depend on. The concrete
// *repository.TodoRepository satisfies it; tests use a fake.
type TodoStore interface {
	FindAll(ctx context.Context) ([]model.Todo, error)
	FindByID(ctx context.Context, id uuid.UUID) (model.Todo, error)
	Create(ctx context.Context, title string, description *string) (model.Todo, error)
	Update(ctx context.Context, id uuid.UUID, title string, description *string, completed bool) (model.Todo, error)
	SetCompleted(ctx context.Context, id uuid.UUID, completed bool) (model.Todo, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// TodoHandler holds the HTTP handlers for the todo API. Business/validation
// logic lives here; the repository handles persistence.
type TodoHandler struct {
	repo TodoStore
}

func NewTodoHandler(repo TodoStore) *TodoHandler {
	return &TodoHandler{repo: repo}
}

// --- helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, errCategory, message string, fieldErrors map[string]string) {
	writeJSON(w, status, model.ApiError{
		Timestamp:   time.Now().UTC(),
		Status:      status,
		Error:       errCategory,
		Message:     message,
		FieldErrors: fieldErrors,
	})
}

// validateTitleDescription mirrors the Jakarta Bean Validation rules:
// title required 1..255, description optional <=2000.
func validateTitleDescription(title string, description *string) map[string]string {
	fe := map[string]string{}
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		fe["title"] = "Title must not be empty"
	} else if len(title) > 255 {
		fe["title"] = "Title must be between 1 and 255 characters"
	}
	if description != nil && len(*description) > 2000 {
		fe["description"] = "Description must be at most 2000 characters"
	}
	if len(fe) == 0 {
		return nil
	}
	return fe
}

// --- handlers --------------------------------------------------------------

// GetAll godoc
//
//	@Summary	Get all todos
//	@Tags		todos
//	@Produce	json
//	@Success	200	{array}	model.Todo
//	@Router		/api/todos [get]
func (h *TodoHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	todos, err := h.repo.FindAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal Server Error", "An unexpected error occurred", nil)
		return
	}
	writeJSON(w, http.StatusOK, todos)
}

// GetByID godoc
//
//	@Summary	Get a todo by id
//	@Tags		todos
//	@Produce	json
//	@Param		id	path		string	true	"Todo id (UUID)"
//	@Success	200	{object}	model.Todo
//	@Failure	404	{object}	model.ApiError
//	@Router		/api/todos/{id} [get]
func (h *TodoHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	todo, err := h.repo.FindByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		notFound(w, id)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, todo)
}

// Create godoc
//
//	@Summary	Create a todo
//	@Tags		todos
//	@Accept		json
//	@Produce	json
//	@Param		body	body		model.CreateTodoRequest	true	"New todo"
//	@Success	201		{object}	model.Todo
//	@Failure	400		{object}	model.ApiError
//	@Router		/api/todos [post]
func (h *TodoHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateTodoRequest
	if !decode(w, r, &req) {
		return
	}
	if fe := validateTitleDescription(req.Title, req.Description); fe != nil {
		writeError(w, http.StatusBadRequest, "Validation failed", "Invalid request", fe)
		return
	}
	todo, err := h.repo.Create(r.Context(), req.Title, req.Description)
	if err != nil {
		serverError(w)
		return
	}
	w.Header().Set("Location", "/api/todos/"+todo.ID.String())
	writeJSON(w, http.StatusCreated, todo)
}

// Update godoc
//
//	@Summary	Update a todo
//	@Tags		todos
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"Todo id (UUID)"
//	@Param		body	body		model.UpdateTodoRequest	true	"Updated todo"
//	@Success	200		{object}	model.Todo
//	@Failure	400		{object}	model.ApiError
//	@Failure	404		{object}	model.ApiError
//	@Router		/api/todos/{id} [put]
func (h *TodoHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req model.UpdateTodoRequest
	if !decode(w, r, &req) {
		return
	}
	if fe := validateTitleDescription(req.Title, req.Description); fe != nil {
		writeError(w, http.StatusBadRequest, "Validation failed", "Invalid request", fe)
		return
	}
	todo, err := h.repo.Update(r.Context(), id, req.Title, req.Description, req.Completed)
	if errors.Is(err, repository.ErrNotFound) {
		notFound(w, id)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, todo)
}

// UpdateCompletion godoc
//
//	@Summary	Update only the completion status
//	@Tags		todos
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string							true	"Todo id (UUID)"
//	@Param		body	body		model.UpdateCompletionRequest	true	"Completion flag"
//	@Success	200		{object}	model.Todo
//	@Failure	400		{object}	model.ApiError
//	@Failure	404		{object}	model.ApiError
//	@Router		/api/todos/{id}/complete [patch]
func (h *TodoHandler) UpdateCompletion(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req model.UpdateCompletionRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Completed == nil {
		writeError(w, http.StatusBadRequest, "Validation failed", "Invalid request",
			map[string]string{"completed": "completed must be provided"})
		return
	}
	todo, err := h.repo.SetCompleted(r.Context(), id, *req.Completed)
	if errors.Is(err, repository.ErrNotFound) {
		notFound(w, id)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, todo)
}

// Delete godoc
//
//	@Summary	Delete a todo
//	@Tags		todos
//	@Param		id	path	string	true	"Todo id (UUID)"
//	@Success	204	"No Content"
//	@Failure	404	{object}	model.ApiError
//	@Router		/api/todos/{id} [delete]
func (h *TodoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	err := h.repo.Delete(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		notFound(w, id)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- shared error paths ----------------------------------------------------

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := r.PathValue("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Bad Request", "Invalid value for parameter 'id'", nil)
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "Malformed request",
			"The request body could not be read or is not valid JSON", nil)
		return false
	}
	return true
}

func notFound(w http.ResponseWriter, id uuid.UUID) {
	writeError(w, http.StatusNotFound, "Not Found", "Todo not found with id: "+id.String(), nil)
}

func serverError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "Internal Server Error", "An unexpected error occurred", nil)
}
