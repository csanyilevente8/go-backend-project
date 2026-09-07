package httpapi

import (
	"net/http"
	"os"
	"strings"
)

// NewRouter wires the routes using net/http pattern routing (Go 1.22+),
// matching the Spring Boot REST contract exactly.
func NewRouter(h *TodoHandler) http.Handler {
	mux := http.NewServeMux()

	// Todo API (same paths, methods, and status codes as the Spring backend).
	mux.HandleFunc("GET /api/todos", h.GetAll)
	mux.HandleFunc("POST /api/todos", h.Create)
	mux.HandleFunc("GET /api/todos/{id}", h.GetByID)
	mux.HandleFunc("PUT /api/todos/{id}", h.Update)
	mux.HandleFunc("PATCH /api/todos/{id}/complete", h.UpdateCompletion)
	mux.HandleFunc("DELETE /api/todos/{id}", h.Delete)

	// Health endpoint kept at the same path so the K8s probes and Ingress
	// are unchanged.
	mux.HandleFunc("GET /actuator/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "UP"})
	})

	return corsMiddleware(mux)
}

// corsMiddleware mirrors the Spring CORS config: allow the configured origin(s)
// (CORS_ALLOWED_ORIGINS, default http://localhost:4200) for the API, with the
// same methods and credentials support.
func corsMiddleware(next http.Handler) http.Handler {
	allowed := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowed == "" {
		allowed = "http://localhost:4200"
	}
	origins := strings.Split(allowed, ",")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && originAllowed(origin, origins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "*")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if strings.TrimSpace(a) == origin {
			return true
		}
	}
	return false
}
