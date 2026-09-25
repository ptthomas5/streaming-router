package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestDispatcherSwap(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/health", "health-v1"); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	d := NewDispatcher(rt)
	d.HandleFunc("health-v1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	d.HandleFunc("health-v2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	next := New()
	if err := next.Handle("GET", "/health", "health-v2"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	d.Swap(next)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status after Swap = %d, want %d", rec.Code, http.StatusTeapot)
	}

	// The original table is untouched by the swap.
	if _, _, ok := rt.Match("GET", "/health"); !ok {
		t.Fatal("Swap mutated the table that was passed to NewDispatcher")
	}
}

func TestDispatcherReload(t *testing.T) {
	rt := New()
	d := NewDispatcher(rt)
	d.HandleFunc("user-show", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	manifest := "GET /users/:id user-show\n"
	n, err := d.Reload(strings.NewReader(manifest))
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if n != 1 {
		t.Fatalf("Reload loaded %d routes, want 1", n)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/9", nil)
	d.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status after Reload = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDispatcherReloadErrorKeepsOldTable(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/health", "health-check"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	d := NewDispatcher(rt)
	d.HandleFunc("health-check", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	_, err := d.Reload(strings.NewReader("GET onlytwofields\n"))
	if err == nil {
		t.Fatal("Reload: expected error for malformed manifest, got nil")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	d.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status after failed Reload = %d, want %d (old table should still serve)", rec.Code, http.StatusOK)
	}
}

func TestDispatcherRedirectTrailingSlashAdded(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/users/", "user-list"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	d := NewDispatcher(rt)
	d.RedirectTrailingSlash = true
	d.HandleFunc("user-list", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?active=true", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if got, want := rec.Header().Get("Location"), "/users/?active=true"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestDispatcherRedirectTrailingSlashRemoved(t *testing.T) {
	rt := New()
	if err := rt.Handle("POST", "/users", "user-create"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	d := NewDispatcher(rt)
	d.RedirectTrailingSlash = true
	d.HandleFunc("user-create", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users/", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusPermanentRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusPermanentRedirect)
	}
	if got, want := rec.Header().Get("Location"), "/users"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestDispatcherRedirectTrailingSlashDisabledByDefault(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/users/", "user-list"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	d := NewDispatcher(rt)
	d.HandleFunc("user-list", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDispatcherRedirectTrailingSlashNoAlternate(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/health", "health-check"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	d := NewDispatcher(rt)
	d.RedirectTrailingSlash = true
	d.HandleFunc("health-check", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	d.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDispatcherSwapConcurrentWithRequests(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/health", "health-check"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	d := NewDispatcher(rt)
	d.HandleFunc("health-check", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			next := New()
			if err := next.Handle("GET", "/health", "health-check"); err != nil {
				t.Errorf("Handle: %v", err)
				return
			}
			d.Swap(next)
		}
	}()

	for i := 0; i < 200; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		d.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	}
	close(stop)
	wg.Wait()
}
