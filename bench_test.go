package router

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
)

// buildBenchTable registers n static routes plus one param route and one
// wildcard route, mimicking a service-registry-sized table where most
// entries are static and only a handful are parameterized.
func buildBenchTable(n int) *Table {
	t := New()
	for i := 0; i < n; i++ {
		pattern := "/services/" + strconv.Itoa(i) + "/status"
		if err := t.Handle("GET", pattern, "status-"+strconv.Itoa(i)); err != nil {
			panic(err)
		}
	}
	if err := t.Handle("GET", "/services/:id/profile", "service-profile"); err != nil {
		panic(err)
	}
	if err := t.Handle("GET", "/assets/*path", "asset-file"); err != nil {
		panic(err)
	}
	return t
}

func BenchmarkMatchStatic(b *testing.B) {
	const n = 50000
	t := buildBenchTable(n)
	path := "/services/" + strconv.Itoa(n-1) + "/status"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, ok := t.Match("GET", path); !ok {
			b.Fatal("expected match")
		}
	}
}

func BenchmarkMatchParam(b *testing.B) {
	const n = 50000
	t := buildBenchTable(n)
	path := "/services/" + strconv.Itoa(n-1) + "/profile"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, ok := t.Match("GET", path); !ok {
			b.Fatal("expected match")
		}
	}
}

func BenchmarkMatchWildcard(b *testing.B) {
	const n = 50000
	t := buildBenchTable(n)
	path := "/assets/js/vendor/react.min.js"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, ok := t.Match("GET", path); !ok {
			b.Fatal("expected match")
		}
	}
}

func BenchmarkMatchNoRoute(b *testing.B) {
	const n = 50000
	t := buildBenchTable(n)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, ok := t.Match("GET", "/nowhere"); ok {
			b.Fatal("expected no match")
		}
	}
}

// buildManifest renders n static routes plus a trailing param route as a
// manifest, the same shape LoadRoutes expects to stream from a file.
func buildManifest(n int) []byte {
	var buf bytes.Buffer
	for i := 0; i < n; i++ {
		fmt.Fprintf(&buf, "GET /services/%d/status status-%d\n", i, i)
	}
	fmt.Fprintf(&buf, "GET /services/:id/profile service-profile\n")
	return buf.Bytes()
}

// BenchmarkLoadRoutes measures the streaming loader on a manifest large
// enough that buffering it whole would show up in a memory profile.
func BenchmarkLoadRoutes(b *testing.B) {
	const n = 50000
	manifest := buildManifest(n)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		t := New()
		r := bytes.NewReader(manifest)
		b.StartTimer()

		if _, err := LoadRoutes(t, r); err != nil {
			b.Fatal(err)
		}
	}
}
