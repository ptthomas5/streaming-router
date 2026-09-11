package router

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"
)

type contextKey int

const paramsContextKey contextKey = 0

// ParamsFromContext returns the Params captured by the route that matched
// the request currently being served, or nil if the matched pattern
// captured nothing (or the request wasn't served through a Dispatcher).
func ParamsFromContext(ctx context.Context) Params {
	p, _ := ctx.Value(paramsContextKey).(Params)
	return p
}

// Dispatcher adapts a Table to http.Handler. A Table only knows how to turn
// a method+path into a tag; Dispatcher closes the loop by mapping each tag
// to the http.Handler that should serve it, since a manifest-loaded table
// never carries Go handler values itself.
type Dispatcher struct {
	table    atomic.Pointer[Table]
	handlers map[string]http.Handler

	// NotFound is used when the path doesn't match any route, or matches
	// a route whose tag has no registered handler. If nil, http.NotFound
	// is used.
	NotFound http.Handler
}

// NewDispatcher returns a Dispatcher backed by t. Callers that need to
// change routes while the server is running should not mutate t further;
// build a new Table and call Swap or Reload instead, since t may still be
// in use by a request that is being served concurrently.
func NewDispatcher(t *Table) *Dispatcher {
	d := &Dispatcher{handlers: make(map[string]http.Handler)}
	d.table.Store(t)
	return d
}

// Swap replaces the table used for requests received after Swap returns.
// Requests already in ServeHTTP keep using the table they looked up; there
// is no lock held across a request, so a swap never blocks or is blocked by
// in-flight traffic.
func (d *Dispatcher) Swap(t *Table) {
	d.table.Store(t)
}

// Reload builds a new Table from the manifest read from r with LoadRoutes
// and, on success, Swaps it in atomically. If r contains a malformed line or
// a conflicting route, Reload returns the error from LoadRoutes and leaves
// the table currently serving requests untouched.
func (d *Dispatcher) Reload(r io.Reader) (int, error) {
	t := New()
	n, err := LoadRoutes(t, r)
	if err != nil {
		return n, err
	}
	d.Swap(t)
	return n, nil
}

// Handle associates tag with h. tag is whatever string was passed as the
// third argument to Table.Handle, or the third field of a manifest line.
func (d *Dispatcher) Handle(tag string, h http.Handler) {
	d.handlers[tag] = h
}

// HandleFunc is Handle for a plain function instead of an http.Handler.
func (d *Dispatcher) HandleFunc(tag string, h http.HandlerFunc) {
	d.Handle(tag, h)
}

// ServeHTTP implements http.Handler. On a match, it stores the captured
// Params in the request context, retrievable with ParamsFromContext, and
// delegates to the handler registered for the matched tag.
func (d *Dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tag, params, ok := d.table.Load().Match(r.Method, r.URL.Path)
	if !ok {
		d.serveNotFound(w, r)
		return
	}
	h, ok := d.handlers[tag]
	if !ok {
		d.serveNotFound(w, r)
		return
	}
	if params != nil {
		r = r.WithContext(context.WithValue(r.Context(), paramsContextKey, params))
	}
	h.ServeHTTP(w, r)
}

func (d *Dispatcher) serveNotFound(w http.ResponseWriter, r *http.Request) {
	if d.NotFound != nil {
		d.NotFound.ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}
