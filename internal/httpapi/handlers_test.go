package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/csanyilevente8/go-backend-project/internal/model"
	"github.com/csanyilevente8/go-backend-project/internal/repository"
)

// fakeStore is an in-memory TodoStore for handler tests (like the mocked
// repository in the Spring controller tests).
type fakeStore struct {
	findAll      func(ctx context.Context) ([]model.Todo, error)
	findByID     func(ctx context.Context, id uuid.UUID) (model.Todo, error)
	create       func(ctx context.Context, title string, description *string) (model.Todo, error)
	update       func(ctx context.Context, id uuid.UUID, title string, description *string, completed bool) (model.Todo, error)
	setCompleted func(ctx context.Context, id uuid.UUID, completed bool) (model.Todo, error)
	del          func(ctx context.Context, id uuid.UUID) error
}

func (f *fakeStore) FindAll(ctx context.Context) ([]model.Todo, error) { return f.findAll(ctx) }
func (f *fakeStore) FindByID(ctx context.Context, id uuid.UUID) (model.Todo, error) {
	return f.findByID(ctx, id)
}
func (f *fakeStore) Create(ctx context.Context, title string, description *string) (model.Todo, error) {
	return f.create(ctx, title, description)
}
func (f *fakeStore) Update(ctx context.Context, id uuid.UUID, title string, description *string, completed bool) (model.Todo, error) {
	return f.update(ctx, id, title, description, completed)
}
func (f *fakeStore) SetCompleted(ctx context.Context, id uuid.UUID, completed bool) (model.Todo, error) {
	return f.setCompleted(ctx, id, completed)
}
func (f *fakeStore) Delete(ctx context.Context, id uuid.UUID) error { return f.del(ctx, id) }

func sample(id uuid.UUID, title string, completed bool) model.Todo {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	desc := "desc"
	return model.Todo{ID: id, Title: title, Description: &desc, Completed: completed, CreatedAt: now, UpdatedAt: now}
}

func newServer(store TodoStore) http.Handler {
	return NewRouter(NewTodoHandler(store))
}

func do(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

func TestGetAll(t *testing.T) {
	store := &fakeStore{findAll: func(ctx context.Context) ([]model.Todo, error) {
		return []model.Todo{sample(uuid.New(), "A", false), sample(uuid.New(), "B", true)}, nil
	}}
	w := do(t, newServer(store), http.MethodGet, "/api/todos", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got []model.Todo
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 todos, got %d", len(got))
	}
}

func TestCreate_Returns201(t *testing.T) {
	id := uuid.New()
	store := &fakeStore{create: func(ctx context.Context, title string, description *string) (model.Todo, error) {
		return sample(id, title, false), nil
	}}
	w := do(t, newServer(store), http.MethodPost, "/api/todos", `{"title":"Learn Go","description":"x"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/api/todos/"+id.String() {
		t.Fatalf("unexpected Location: %q", loc)
	}
}

func TestCreate_BlankTitle_Returns400(t *testing.T) {
	store := &fakeStore{}
	w := do(t, newServer(store), http.MethodPost, "/api/todos", `{"title":"","description":"x"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	var e model.ApiError
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.FieldErrors["title"] == "" {
		t.Fatalf("expected a title field error, got %+v", e)
	}
}

func TestCreate_TitleTooLong_Returns400(t *testing.T) {
	store := &fakeStore{}
	long := strings.Repeat("x", 256)
	w := do(t, newServer(store), http.MethodPost, "/api/todos", `{"title":"`+long+`"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestGetByID_NotFound_Returns404(t *testing.T) {
	store := &fakeStore{findByID: func(ctx context.Context, id uuid.UUID) (model.Todo, error) {
		return model.Todo{}, repository.ErrNotFound
	}}
	w := do(t, newServer(store), http.MethodGet, "/api/todos/"+uuid.New().String(), "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestGetByID_BadUUID_Returns400(t *testing.T) {
	store := &fakeStore{}
	w := do(t, newServer(store), http.MethodGet, "/api/todos/not-a-uuid", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestUpdate_Returns200(t *testing.T) {
	id := uuid.New()
	store := &fakeStore{update: func(ctx context.Context, gotID uuid.UUID, title string, description *string, completed bool) (model.Todo, error) {
		return sample(gotID, title, completed), nil
	}}
	w := do(t, newServer(store), http.MethodPut, "/api/todos/"+id.String(), `{"title":"U","description":"y","completed":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestUpdateCompletion_MissingField_Returns400(t *testing.T) {
	store := &fakeStore{}
	w := do(t, newServer(store), http.MethodPatch, "/api/todos/"+uuid.New().String()+"/complete", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestUpdateCompletion_Returns200(t *testing.T) {
	id := uuid.New()
	store := &fakeStore{setCompleted: func(ctx context.Context, gotID uuid.UUID, completed bool) (model.Todo, error) {
		return sample(gotID, "A", completed), nil
	}}
	w := do(t, newServer(store), http.MethodPatch, "/api/todos/"+id.String()+"/complete", `{"completed":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestDelete_Returns204(t *testing.T) {
	store := &fakeStore{del: func(ctx context.Context, id uuid.UUID) error { return nil }}
	w := do(t, newServer(store), http.MethodDelete, "/api/todos/"+uuid.New().String(), "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
}

func TestDelete_NotFound_Returns404(t *testing.T) {
	store := &fakeStore{del: func(ctx context.Context, id uuid.UUID) error { return repository.ErrNotFound }}
	w := do(t, newServer(store), http.MethodDelete, "/api/todos/"+uuid.New().String(), "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestHealth(t *testing.T) {
	w := do(t, newServer(&fakeStore{}), http.MethodGet, "/actuator/health", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "UP") {
		t.Fatalf("health not UP: %d %s", w.Code, w.Body.String())
	}
}

func TestMalformedJSON_Returns400(t *testing.T) {
	w := do(t, newServer(&fakeStore{}), http.MethodPost, "/api/todos", `{not json`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}
