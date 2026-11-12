# Rate limits, retries and monitoring

QuickBase allows **100 JSON API requests per 10 seconds per user token** ([rate limiting](https://developer.quickbase.com/rateLimit)). The XML API limits separately: 10 requests per second per table.

## Options

```go
quickbase.New("myrealm",
	quickbase.WithUserToken("token"),
	quickbase.WithMaxRetries(5),                 // attempts in all, default 3
	quickbase.WithRetryDelay(time.Second),       // first backoff, default 1s
	quickbase.WithMaxRetryDelay(30*time.Second), // backoff cap, default 30s
	quickbase.WithTimeout(60*time.Second),       // per request, default 30s
	quickbase.WithProactiveThrottle(100),        // at most 100 requests per 10s
	quickbase.WithMaxIdleConnsPerHost(10),       // default 6
	quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
		log.Printf("%s %s -> %d (%s)", info.Method, info.Path, info.StatusCode, info.Duration)
	}),
	quickbase.WithOnRetry(func(info quickbase.RetryInfo) {
		log.Printf("retrying %s (attempt %d, %s, in %s)", info.Path, info.Attempt, info.Reason, info.WaitTime)
	}),
	quickbase.WithOnRateLimit(func(info quickbase.RateLimitInfo) {
		log.Printf("rate limited; retry after %ds", info.RetryAfter)
	}),
)
```

| Option | Default | Does |
|---|---|---|
| `WithMaxRetries(n)` | 3 | Attempts in all, counting the first; below 1 means one |
| `WithRetryDelay(d)` | 1s | The first backoff |
| `WithBackoffMultiplier(m)` | 2 | Growth per attempt |
| `WithMaxRetryDelay(d)` | 30s | The backoff cap |
| `WithTimeout(d)` | 30s | Each request |
| `WithProactiveThrottle(n)` | off | At most `n` requests in any 10 seconds |
| `WithMaxIdleConnsPerHost(n)` | 6 | Connections kept for reuse (Go's own default is 2) |
| `WithMaxIdleConns(n)` | 100 | Connections kept for reuse in all |
| `WithIdleConnTimeout(d)` | 90s | How long an idle connection is kept |
| `WithDebug(true)` | off | Log requests and retries |

## What is retried

| Failure | Reads (GETs, and POSTs that read: `RunQuery`, `RunReport`, `RunFormula`, ...) | Writes (upserts, creates, deletes, XML write actions) |
|---|---|---|
| 429 | retried | retried: QuickBase didn't process it |
| 401 | retried when the auth strategy has a new token | the same |
| Connection never opened (DNS, refused) | retried | retried: it wasn't sent |
| Timeout, 5xx, connection lost | retried | **not retried**: returned |

A write that timed out or got a 502 may have been applied, and sending it again could, for example, add an upsert's records twice. Check what happened (query for the records) before sending it again.

A retried failure is retried until the attempts run out. The wait is QuickBase's `Retry-After` when it sends one; otherwise `RetryDelay × Multiplier^(attempt−1)`, ±10% jitter, capped at `MaxRetryDelay`. After the last attempt a 429 returns `*RateLimitError`, whose `RateLimitInfo` has the `Retry-After` value and QuickBase's ray IDs for support requests.

## Throttling

Retries recover from a 429; throttling avoids it. `WithProactiveThrottle(100)` keeps a sliding 10-second window and waits before a request that would exceed it. It is sized for the JSON API's limit; the XML API's per-table limit isn't modelled.

For batch work, raise concurrency and throttle together: `WithMaxIdleConnsPerHost(10)` lets ten requests run at once, and `WithProactiveThrottle(100)` spreads them across the window. Without the throttle, parallel requests burst past the limit, get 429s and wait.

## Monitoring

`WithOnRequest` sees every request: `Method`, `Path`, `StatusCode` (0 for a network error), `Duration`, `Attempt` (1 for the first try), `Error` and `RequestBody`. `WithOnRetry` sees each retry before its wait: `Method`, `Path`, `Attempt` (the one about to run), `Reason` (`"429"`, `"503"`, `"network error"`, ...) and `WaitTime`. `WithOnRateLimit` sees each 429.

With Prometheus:

```go
requestDuration := prometheus.NewHistogramVec(
	prometheus.HistogramOpts{Name: "quickbase_request_duration_seconds"},
	[]string{"method", "status"},
)
retries := prometheus.NewCounterVec(
	prometheus.CounterOpts{Name: "quickbase_retries_total"},
	[]string{"reason"},
)

quickbase.New("myrealm",
	quickbase.WithUserToken("token"),
	quickbase.WithOnRequest(func(info quickbase.RequestInfo) {
		requestDuration.WithLabelValues(info.Method, strconv.Itoa(info.StatusCode)).Observe(info.Duration.Seconds())
	}),
	quickbase.WithOnRetry(func(info quickbase.RetryInfo) {
		retries.WithLabelValues(info.Reason).Inc()
	}),
)
```

Label by path only after replacing IDs in it, or every app and table becomes its own series.

## Values QuickBase rejects

An upsert whose values QuickBase rejects (a date in the wrong form, text in a numeric field) still saves the records, with those fields blank, and answers 207. The result's `Metadata().LineErrors` has the rejections, keyed by each record's 1-based position. `WithLineErrorsAsErrors()` returns them as a `*LineErrorsError` too, alongside the result, so the saved records' IDs aren't lost:

```go
result, err := client.Upsert(tableID).Data(records...).Run(ctx)
var rejected *quickbase.LineErrorsError
if errors.As(err, &rejected) {
	log.Printf("saved %v; rejected %v", result.Metadata().CreatedRecordIds(), rejected.LineErrors)
}
```
