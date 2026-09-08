# streaming-router

A small Go library for matching HTTP method + path pairs against a table of
routes. It doesn't wrap `net/http` or own a handler type — `Match` gives you
back a string tag and any captured parameters, and you decide what to do
with the tag. That split exists because of the second thing this library
does: it can build the route table from a manifest file instead of Go code,
and a manifest has no way to hand you a function pointer.

## The problem

Most Go routers assume you register routes by calling a method for each one,
in code, at startup. That's fine until the route table itself is generated
somewhere else — a build step that scans a monorepo for services, a sync job
that pulls routes from a service registry, a deploy that writes out however
many thousand entries a large system ends up with. At that point the table
is a file, and the naive way to load it is:

```go
data, _ := os.ReadFile("routes.manifest")
// parse data, all of it, in memory, at once
```

That works until the manifest is big enough that reading it whole is the
first thing that shows up in a memory profile. `LoadRoutes` reads the
manifest a line at a time with a `bufio.Scanner`, so the memory it uses
doesn't grow with the size of the file — it only depends on the longest
single line.

## Usage

Registering routes directly, the way you'd expect:

```go
rt := router.New()
rt.Handle("GET", "/", "home")
rt.Handle("GET", "/users/:id", "user-show")
rt.Handle("GET", "/static/*path", "static-file")

tag, params, ok := rt.Match("GET", "/users/42")
// tag == "user-show", params["id"] == "42", ok == true
```

Loading a route table from a manifest, streamed instead of buffered:

```go
f, err := os.Open("routes.manifest")
if err != nil {
    log.Fatal(err)
}
defer f.Close()

rt := router.New()
n, err := router.LoadRoutes(rt, f)
if err != nil {
    log.Fatalf("loading routes: %v", err)
}
log.Printf("loaded %d routes", n)
```

A manifest is plain text, one route per line:

```
# method  pattern              tag
GET       /                    home
GET       /users/:id           user-show
GET       /users/:id/posts/:postID  user-post-show
GET       /static/*path        static-file
POST      /users               user-create
```

Blank lines and lines starting with `#` are skipped. Fields are separated by
whitespace; the tag is whatever string your application uses to look up the
real handler, e.g. a key into a `map[string]http.HandlerFunc` you build
separately.

## Pattern syntax

- `/users` — matches exactly `/users`.
- `/users/:id` — `:id` captures one path segment.
- `/static/*path` — `*path` must be the last segment and captures the rest
  of the path, including any slashes in it.

## Serving with net/http

`Table.Match` returns a tag, not a handler, so it needs no wrapping to stay
usable outside `net/http` at all. `Dispatcher` is the adapter for when you
do want an `http.Handler`: it maps each tag to a handler and does the
`Match` call for you.

```go
rt := router.New()
rt.Handle("GET", "/users/:id", "user-show")

d := router.NewDispatcher(rt)
d.HandleFunc("user-show", func(w http.ResponseWriter, r *http.Request) {
    id := router.ParamsFromContext(r.Context())["id"]
    fmt.Fprintf(w, "user %s", id)
})

http.ListenAndServe(":8080", d)
```

Tags with no registered handler, and paths with no matching route, both
fall through to `d.NotFound` if set, or `http.NotFound` otherwise.

## Status

The matching tree, the streaming loader, and the net/http adapter work,
are tested, and have benchmarks covering static, param, and wildcard
matches plus `LoadRoutes` on a large manifest (`go test -bench .`). Route
priority is already fixed by construction: static beats param beats
wildcard at each segment, and `Handle` rejects anything that would make
that ambiguous. Still missing: atomic table reloads, a manifest writer,
and query string / trailing-slash redirect handling.

## License

MIT, see [LICENSE](LICENSE).
