package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csanyilevente8/go-backend-project/internal/events"
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

// ActivityStore is the read side used by GET /api/activity.
type ActivityStore interface {
	FindRecent(ctx context.Context, limit int) ([]model.Activity, error)
}

// NotificationStore is the read side used by the /api/notifications endpoints.
// It is fed by the "notifier" consumer group (fan-out, independent of activity).
type NotificationStore interface {
	FindRecent(ctx context.Context, limit int) ([]model.Notification, error)
	CountUnread(ctx context.Context) (int, error)
	MarkAllRead(ctx context.Context) (int64, error)
}

// TodoHandler holds the HTTP handlers for the todo API. Business/validation
// logic lives here; the repository handles persistence. After a successful DB
// mutation it publishes a domain event (fire-and-forget) via the publisher.
type TodoHandler struct {
	repo          TodoStore
	activity      ActivityStore
	notifications NotificationStore
	publisher     events.Publisher
}

func NewTodoHandler(repo TodoStore, activity ActivityStore, notifications NotificationStore, publisher events.Publisher) *TodoHandler {
	if publisher == nil {
		publisher = events.NoopPublisher{}
	}
	return &TodoHandler{repo: repo, activity: activity, notifications: notifications, publisher: publisher}
}

// emit publishes a todo event. Best-effort: never affects the response.
func (h *TodoHandler) emit(ctx context.Context, typ string, t model.Todo) {
	h.publisher.Publish(ctx, events.TodoEvent{
		Type:      typ,
		TodoID:    t.ID,
		Title:     t.Title,
		Completed: t.Completed,
		Timestamp: time.Now().UTC(),
	})
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
	h.emit(r.Context(), events.TypeCreated, todo)
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
	h.emit(r.Context(), events.TypeUpdated, todo)
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
	h.emit(r.Context(), events.TypeCompleted, todo)
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
	// Fetch first so the event can carry the title (best-effort).
	existing, _ := h.repo.FindByID(r.Context(), id)
	err := h.repo.Delete(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		notFound(w, id)
		return
	}
	if err != nil {
		serverError(w)
		return
	}
	existing.ID = id
	h.emit(r.Context(), events.TypeDeleted, existing)
	w.WriteHeader(http.StatusNoContent)
}

// GetActivity godoc
//
//	@Summary	Recent activity (from the Kafka-fed activity log)
//	@Tags		activity
//	@Produce	json
//	@Param		limit	query	int	false	"Max entries (default 100)"
//	@Success	200	{array}	model.Activity
//	@Router		/api/activity [get]
func (h *TodoHandler) GetActivity(w http.ResponseWriter, r *http.Request) {
	if h.activity == nil {
		writeJSON(w, http.StatusOK, []model.Activity{})
		return
	}
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	items, err := h.activity.FindRecent(r.Context(), limit)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetNotifications godoc
//
//	@Summary	Recent notifications (from the Kafka "notifier" consumer group)
//	@Tags		notifications
//	@Produce	json
//	@Param		limit	query	int	false	"Max entries (default 100)"
//	@Success	200	{array}	model.Notification
//	@Router		/api/notifications [get]
func (h *TodoHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusOK, []model.Notification{})
		return
	}
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	items, err := h.notifications.FindRecent(r.Context(), limit)
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetUnreadCount godoc
//
//	@Summary	Number of unread notifications (for the UI bell badge)
//	@Tags		notifications
//	@Produce	json
//	@Success	200	{object}	map[string]int
//	@Router		/api/notifications/unread-count [get]
func (h *TodoHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusOK, map[string]int{"count": 0})
		return
	}
	n, err := h.notifications.CountUnread(r.Context())
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": n})
}

// MarkNotificationsRead godoc
//
//	@Summary	Mark all notifications as read
//	@Tags		notifications
//	@Produce	json
//	@Success	200	{object}	map[string]int64
//	@Router		/api/notifications/read [post]
func (h *TodoHandler) MarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if h.notifications == nil {
		writeJSON(w, http.StatusOK, map[string]int64{"updated": 0})
		return
	}
	n, err := h.notifications.MarkAllRead(r.Context())
	if err != nil {
		serverError(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"updated": n})
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
