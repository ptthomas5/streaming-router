// Package router matches HTTP method+path pairs against a table of patterns
// and returns an opaque tag for the caller to dispatch on. It does not know
// about http.Handler on purpose: the table can be built from a source that
// has no Go functions to hand out (see LoadRoutes), so the only thing a
// route carries is a string tag that the caller maps to real behavior.
package router

import (
	"errors"
	"net/url"
	"strings"
)

// Params holds the values captured from named segments and the wildcard,
// if any, of the pattern that matched a path.
type Params map[string]string

// ErrConflict is returned by Handle when a pattern is registered twice for
// the same method, or when two patterns would ambiguously claim the same
// wildcard position.
var ErrConflict = errors.New("router: conflicting route")

type node struct {
	static       map[string]*node
	param        *node
	paramName    string
	wildcard     *node
	wildcardName string
	tag          string
	hasTag       bool
}

func newNode() *node {
	return &node{static: make(map[string]*node)}
}

// Table is a set of routes, indexed by HTTP method. The zero value is not
// usable; construct one with New.
type Table struct {
	roots map[string]*node
}

// New returns an empty routing table.
func New() *Table {
	return &Table{roots: make(map[string]*node)}
}

// Handle registers pattern under method, associating it with tag. Patterns
// are slash-separated segments. A segment starting with ':' captures the
// segment value under that name (e.g. ":id"). A segment starting with '*'
// must be the last segment and captures the remainder of the path,
// including slashes (e.g. "*rest").
//
// Handle returns ErrConflict if the exact pattern is already registered for
// method, or if it collides with an existing wildcard/param at the same
// position.
func (t *Table) Handle(method, pattern, tag string) error {
	if method == "" {
		return errors.New("router: empty method")
	}
	if !strings.HasPrefix(pattern, "/") {
		return errors.New("router: pattern must start with /")
	}
	segs := splitPath(pattern)

	root, ok := t.roots[method]
	if !ok {
		root = newNode()
		t.roots[method] = root
	}

	n := root
	for i, seg := range segs {
		switch {
		case strings.HasPrefix(seg, "*"):
			if i != len(segs)-1 {
				return errors.New("router: wildcard must be the last segment")
			}
			name := seg[1:]
			if n.wildcard == nil {
				n.wildcard = newNode()
				n.wildcardName = name
			} else if n.wildcardName != name {
				return ErrConflict
			}
			n = n.wildcard

		case strings.HasPrefix(seg, ":"):
			name := seg[1:]
			if n.param == nil {
				n.param = newNode()
				n.paramName = name
			} else if n.paramName != name {
				return ErrConflict
			}
			n = n.param

		default:
			child, ok := n.static[seg]
			if !ok {
				child = newNode()
				n.static[seg] = child
			}
			n = child
		}
	}

	if n.hasTag {
		return ErrConflict
	}
	n.tag = tag
	n.hasTag = true
	return nil
}

// Match looks up the route registered for method and path. path may carry a
// query string ("/users/42?active=true"); it is stripped before matching so
// it never affects which route is chosen, and its values are parsed into
// the returned Params under their own keys. A query key that collides with
// a captured path parameter is ignored, so a route's own segments always
// win. ok is false if no route matches. The returned Params is nil when the
// matched pattern captured nothing and the query string was empty or absent,
// so callers can range over it without a nil check.
func (t *Table) Match(method, path string) (tag string, params Params, ok bool) {
	root, exists := t.roots[method]
	if !exists {
		return "", nil, false
	}
	path, rawQuery := splitQuery(path)
	segs := splitPath(path)
	tag, params, ok = matchNode(root, segs, nil)
	if !ok {
		return "", nil, false
	}
	if rawQuery != "" {
		params = mergeQuery(params, rawQuery)
	}
	return tag, params, true
}

// splitQuery separates a path from its query string at the first '?', the
// same split net/url does for a request URI. The query string itself is
// returned without the leading '?'.
func splitQuery(path string) (string, string) {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		return path[:i], path[i+1:]
	}
	return path, ""
}

// mergeQuery parses rawQuery and adds each key to params, keeping the first
// value of a repeated key the way url.Values.Get does. A malformed query
// string (invalid percent-encoding) is treated as empty rather than turning
// a route match into an error, since the path already matched on its own.
func mergeQuery(params Params, rawQuery string) Params {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return params
	}
	for k, v := range values {
		if len(v) == 0 {
			continue
		}
		if _, exists := params[k]; exists {
			continue
		}
		params = addParam(params, k, v[0])
	}
	return params
}

func matchNode(n *node, segs []string, params Params) (string, Params, bool) {
	if len(segs) == 0 {
		if n.hasTag {
			return n.tag, params, true
		}
		return "", nil, false
	}

	seg, rest := segs[0], segs[1:]

	if child, ok := n.static[seg]; ok {
		if tag, p, ok := matchNode(child, rest, params); ok {
			return tag, p, true
		}
	}

	if n.param != nil {
		if tag, p, ok := matchNode(n.param, rest, addParam(params, n.paramName, seg)); ok {
			return tag, p, true
		}
	}

	if n.wildcard != nil && n.wildcard.hasTag {
		value := strings.Join(segs, "/")
		return n.wildcard.tag, addParam(params, n.wildcardName, value), true
	}

	return "", nil, false
}

func addParam(params Params, name, value string) Params {
	if params == nil {
		params = make(Params, 4)
	}
	params[name] = value
	return params
}

// splitPath splits a "/"-separated path into non-empty segments. A bare "/"
// yields an empty slice, matching the root route registered as "/".
func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
