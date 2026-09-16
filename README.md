# go_practices

Four exercises I wrote to learn Go. Each one is a self-contained `main` package
aimed at a different part of the language and standard library, picked so that
between them they cover most of what you actually reach for.

| | What I was learning |
| --- | --- |
| `practice1` | Errors and the basics. Parsing a ledger of dated expense entries, with a custom error type that carries the line it failed on, plus `sort` and aggregation. First go at `context` for cancellation. |
| `practice2` | Concurrency. A structured log parser — timestamps, levels, service names, latencies — processing lines across goroutines coordinated with `sync`. |
| `practice3` | The standard library as a server. `net/http` over a mutex-guarded in-memory cache with TTL expiry, JSON encoding, and graceful shutdown on SIGINT/SIGTERM through `os/signal`. |
| `practice4` | Generics and testing. A `Set[T comparable]` on top of `map[T]struct{}`, driven by table tests. |

Things that took the longest to get comfortable with, in case it's useful to
anyone else starting: that errors are values you construct and inspect rather
than throw, that a `struct{}` map value costs nothing and is the idiomatic set,
and that `defer cancel()` belongs immediately after every `context.WithCancel`
whether or not you think you need it yet.

## Running

```sh
cd practice1 && go run .
cd practice4 && go test ./...
```
