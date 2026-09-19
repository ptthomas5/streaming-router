package router

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteRoutesRoundTrip(t *testing.T) {
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

	var buf bytes.Buffer
	if err := WriteRoutes(&buf, rt); err != nil {
		t.Fatalf("WriteRoutes: %v", err)
	}

	rebuilt := New()
	n, err := LoadRoutes(rebuilt, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("LoadRoutes(written manifest): %v", err)
	}
	if n != 5 {
		t.Fatalf("LoadRoutes loaded %d routes, want 5", n)
	}

	cases := []struct {
		method, path string
		wantTag      string
	}{
		{"GET", "/", "home"},
		{"GET", "/users/42", "user-show"},
		{"GET", "/users/42/posts/7", "user-post-show"},
		{"GET", "/static/css/site.css", "static-file"},
		{"POST", "/users", "user-create"},
	}
	for _, c := range cases {
		tag, _, ok := rebuilt.Match(c.method, c.path)
		if !ok || tag != c.wantTag {
			t.Errorf("Match(%q, %q) after round trip = (%q, %v), want %q", c.method, c.path, tag, ok, c.wantTag)
		}
	}
}

func TestWriteRoutesDeterministicOrder(t *testing.T) {
	rt := New()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
	}
	must(rt.Handle("GET", "/zeta", "zeta"))
	must(rt.Handle("GET", "/alpha", "alpha"))
	must(rt.Handle("POST", "/mid", "mid"))
	must(rt.Handle("GET", "/mid", "get-mid"))

	var first, second bytes.Buffer
	if err := WriteRoutes(&first, rt); err != nil {
		t.Fatalf("WriteRoutes: %v", err)
	}
	if err := WriteRoutes(&second, rt); err != nil {
		t.Fatalf("WriteRoutes: %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("WriteRoutes is not deterministic:\n%s\nvs\n%s", first.String(), second.String())
	}

	want := []string{
		"GET /alpha alpha",
		"GET /mid get-mid",
		"GET /zeta zeta",
		"POST /mid mid",
	}
	got := strings.Split(strings.TrimRight(first.String(), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("WriteRoutes lines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWriteRoutesEmptyTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteRoutes(&buf, New()); err != nil {
		t.Fatalf("WriteRoutes: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("WriteRoutes(empty table) = %q, want empty", buf.String())
	}
}
