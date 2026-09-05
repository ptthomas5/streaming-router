package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDispatcherServeHTTP(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/users/:id", "user-show"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if err := rt.Handle("GET", "/health", "health-check"); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	d := NewDispatcher(rt)
	d.HandleFunc("health-check", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	d.HandleFunc("user-show", func(w http.ResponseWriter, r *http.Request) {
		id := ParamsFromContext(r.Context())["id"]
		w.Header().Set("X-User-ID", id)
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("X-User-ID"); got != "42" {
		t.Fatalf("X-User-ID = %q, want %q", got, "42")
	}
}

func TestDispatcherNoRouteMatch(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/users/:id", "user-show"); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	d := NewDispatcher(rt)
	d.HandleFunc("user-show", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDispatcherUnregisteredTag(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/orphan", "orphan-tag"); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	d := NewDispatcher(rt)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orphan", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDispatcherCustomNotFound(t *testing.T) {
	rt := New()
	d := NewDispatcher(rt)
	d.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

func TestParamsFromContextNoMatch(t *testing.T) {
	if p := ParamsFromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context()); p != nil {
		t.Fatalf("ParamsFromContext on bare context = %v, want nil", p)
	}
}
