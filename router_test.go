package router

import (
	"strings"
	"testing"
)

func TestHandleAndMatch(t *testing.T) {
	rt := New()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
	}
	must(rt.Handle("GET", "/", "home"))
	must(rt.Handle("GET", "/users/:id", "user-show"))
	must(rt.Handle("GET", "/users/:id/posts/:postID", "user-post-show"))
	must(rt.Handle("GET", "/static/*path", "static-file"))
	must(rt.Handle("POST", "/users", "user-create"))

	cases := []struct {
		method, path string
		wantTag      string
		wantParams   Params
		wantOK       bool
	}{
		{"GET", "/", "home", nil, true},
		{"GET", "/users/42", "user-show", Params{"id": "42"}, true},
		{"GET", "/users/42/posts/7", "user-post-show", Params{"id": "42", "postID": "7"}, true},
		{"GET", "/static/css/site.css", "static-file", Params{"path": "css/site.css"}, true},
		{"POST", "/users", "user-create", nil, true},
		{"GET", "/nope", "", nil, false},
		{"DELETE", "/users/42", "", nil, false},
	}

	for _, c := range cases {
		tag, params, ok := rt.Match(c.method, c.path)
		if ok != c.wantOK || tag != c.wantTag {
			t.Errorf("Match(%q, %q) = (%q, %v, %v), want (%q, %v, %v)",
				c.method, c.path, tag, params, ok, c.wantTag, c.wantParams, c.wantOK)
			continue
		}
		if len(params) != len(c.wantParams) {
			t.Errorf("Match(%q, %q) params = %v, want %v", c.method, c.path, params, c.wantParams)
			continue
		}
		for k, v := range c.wantParams {
			if params[k] != v {
				t.Errorf("Match(%q, %q) params[%q] = %q, want %q", c.method, c.path, k, params[k], v)
			}
		}
	}
}

func TestHandleConflict(t *testing.T) {
	rt := New()
	if err := rt.Handle("GET", "/users/:id", "a"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if err := rt.Handle("GET", "/users/:id", "b"); err != ErrConflict {
		t.Fatalf("Handle duplicate: got %v, want ErrConflict", err)
	}
	if err := rt.Handle("GET", "/users/:userID", "c"); err != ErrConflict {
		t.Fatalf("Handle with different param name at same position: got %v, want ErrConflict", err)
	}
}

func TestLoadRoutesStreaming(t *testing.T) {
	manifest := strings.Join([]string{
		"# comment lines and blanks are skipped",
		"",
		"GET /health health-check",
		"GET /users/:id user-show",
		"POST /users user-create",
	}, "\n")

	rt := New()
	n, err := LoadRoutes(rt, strings.NewReader(manifest))
	if err != nil {
		t.Fatalf("LoadRoutes: %v", err)
	}
	if n != 3 {
		t.Fatalf("LoadRoutes loaded %d routes, want 3", n)
	}

	tag, params, ok := rt.Match("GET", "/users/9")
	if !ok || tag != "user-show" || params["id"] != "9" {
		t.Fatalf("Match after LoadRoutes = (%q, %v, %v)", tag, params, ok)
	}
}

func TestLoadRoutesMalformedLine(t *testing.T) {
	rt := New()
	_, err := LoadRoutes(rt, strings.NewReader("GET /ok ok\nGET onlytwofields\n"))
	if err == nil {
		t.Fatal("LoadRoutes: expected error for malformed line, got nil")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("LoadRoutes error = %v, want it to mention line 2", err)
	}
}
