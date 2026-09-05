package router

import (
	"context"
	"net/http"
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
	table    *Table
	handlers map[string]http.Handler

	// NotFound is used when the path doesn't match any route, or matches
	// a route whose tag has no registered handler. If nil, http.NotFound
	// is used.
	NotFound http.Handler
}

// NewDispatcher returns a Dispatcher backed by t. Routes may still be added
// to t after this call; Dispatcher always looks up the current state of the
// table.
func NewDispatcher(t *Table) *Dispatcher {
	return &Dispatcher{
		table:    t,
		handlers: make(map[string]http.Handler),
	}
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
	tag, params, ok := d.table.Match(r.Method, r.URL.Path)
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
