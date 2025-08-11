# utils-golang

Reusable Go utilities. Currently includes:

- `logv2`: Lightweight Google Cloud Logging client for Cloud Functions and services

## Install

```bash
go get github.com/print-engine/utils-golang@latest
```

Or pin a version after tagging a release (recommended):

```bash
go get github.com/print-engine/utils-golang@v0.1.0
```

## logv2

A small, reusable Google Cloud Logging client for Go. It reuses a single `logging.Client`, supports request correlation (trace and execution ID), structured payloads, and optional notifier hooks.

### Quick start (Cloud Function HTTP)

```go
package logv2test

import (
    v2 "github.com/print-engine/utils-golang/logv2"
    "cloud.google.com/go/logging"
    "context"
    "fmt"
    "net/http"
    "os"
)

// LogV2Test is the HTTP entrypoint for a Cloud Function
func LogV2Test(w http.ResponseWriter, r *http.Request) {
    ctx := context.Background()

    lg, err := v2.New(ctx,
        v2.WithInvoker("logv2-test"),
        v2.WithLogName("LogV2_Test"),
        // v2.WithProjectID(os.Getenv("GOOGLE_CLOUD_PROJECT")), // optional; auto-detected if omitted
        v2.WithCommonLabels(map[string]string{
            "service": "logv2-test",
            "env":     os.Getenv("ENVIRONMENT"),
        }),
        // v2.WithNotifier(mySlackNotifier{}, logging.Error), // optional
    )
    if err != nil {
        fmt.Fprintf(w, "failed to init logger: %v", err)
        return
    }
    defer lg.Close()

    // Option A: pass ctx/req explicitly when needed
    lg.Info(ctx, r, "function started")

    // Option B: bind per-request once and use a request-scoped logger
    reqLog := lg.ForRequest(ctx, r)
    reqLog.Debug("debug message", map[string]any{"stage": "start"})
    reqLog.Info("info message")
    reqLog.Warning("warning message", map[string]any{"warn": true})
    reqLog.Error("error message", fmt.Errorf("synthetic error"))

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"ok": true}`))
}

// Example notifier; implements v2.Notifier
type mySlackNotifier struct{}

func (mySlackNotifier) Notify(ctx context.Context, severity logging.Severity, executionID string, message string, payload any) {
    // send to Slack, email, etc.
}
```

### Headers and trace correlation

- To correlate with Cloud Trace, include `X-Cloud-Trace-Context` on the request:
  - Format: `TRACE_ID/SPAN_ID;o=1` (SPAN_ID optional)
  - Example curl:
    ```bash
    curl -H 'X-Cloud-Trace-Context: 105445aa7843bc8bf206b120001000/1;o=1' \
      https://YOUR_REGION-YOUR_PROJECT.cloudfunctions.net/LogV2Test
    ```
- Project ID must be known to include the `trace` field in Cloud Logging:
  - Set `GOOGLE_CLOUD_PROJECT`, or
  - Use `v2.WithProjectID("your-project-id")`, or
  - Let the logger auto-detect on GCE.

### Options

- `WithProjectID(string)`
- `WithLogName(string)`
- `WithInvoker(string)`
- `WithCommonLabels(map[string]string)`
- `WithExecutionIDHeaders(...string)`
- `WithNotifier(Notifier, logging.Severity)`
- `WithStdoutOnly()`

### Behavior

- When running outside GCP without `GOOGLE_CLOUD_PROJECT`, the logger falls back to structured JSON on stdout.
- A single `logging.Client` is reused; call `Close()` to flush on shutdown.
- Error values passed as `data` are stringified for better JSON encoding.

### Local testing tips

- Invoke locally and pass the trace header to see correlation fields populated:
  ```bash
  curl -H 'X-Cloud-Trace-Context: 105445aa7843bc8bf206b120001000/1;o=1' localhost:8080
  ```
- For Cloud Functions, ensure the function runtime has access to the module (public repo or vendor).

### Versioning

- Tags follow SemVer: `v0.1.0`, `v1.0.0`, etc.
- For major versions v2+, the module path must include `/v2` per Go module rules.
