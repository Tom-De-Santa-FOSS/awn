# Error Catalog

Every error returned by awn (CLI, RPC, SDK) is a structured `AwnError` with these fields:

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Machine-readable error code |
| `category` | string | Error category for grouping |
| `message` | string | Human-readable description |
| `retryable` | bool | Whether the operation may succeed on retry |
| `suggestion` | string | Actionable hint for resolution |
| `context` | map | Additional key-value context (optional) |

## Error Codes

| Code | Category | Retryable | When it occurs | Suggestion |
|------|----------|-----------|----------------|------------|
| `SESSION_NOT_FOUND` | session | no | Session ID does not match any active session | Check active sessions with `awn list` |
| `SESSION_EXITED` | session | no | Session's process has terminated | Create a new session with `awn create` |
| `TIMEOUT` | timeout | yes | Wait condition not met within timeout | Increase `--timeout` or check if the condition can occur |
| `VALIDATION_ERROR` | validation | no | Invalid input to a command | Check command usage |
| `DAEMON_NOT_RUNNING` | daemon | yes | Cannot connect to the daemon | Start the daemon with `awn daemon start` |
| `DAEMON_ALREADY_RUNNING` | daemon | no | Trying to start a second daemon | Stop it first with `awn daemon stop` |
| `CONNECTION_FAILED` | transport | yes | WebSocket connection failed | Check that the daemon is running and reachable |
| `AUTH_REQUIRED` | auth | no | TCP mode without `AWN_TOKEN` | Set `AWN_TOKEN` environment variable |
| `AUTH_FAILED` | auth | no | Invalid or mismatched token | Check that `AWN_TOKEN` matches on daemon and client |
| `INVALID_KEY` | input | no | Unrecognized key name in `press` | See supported keys list |
| `PIPELINE_STEP_FAILED` | pipeline | no | A pipeline step encountered an error | Check the `step_index` in context |
| `INVALID_INPUT` | validation | no | Invalid directory or parameter | Check the value provided |

## CLI Error Format

In human mode, errors display as:

```
error: session "abc" not found
hint: check session ID with awn list
```

In `--json` mode, errors are full structured JSON on stderr.

## SDK Error Handling

```go
import "github.com/tom/awn"

err := client.Screenshot(ctx, "bad-id")

// Check error code
if awn.ErrorCode(err) == "SESSION_NOT_FOUND" {
    // handle missing session
}

// Check retryability
if awn.IsRetryable(err) {
    // safe to retry
}

// Unwrap structured error
var ae *awn.AwnError
if errors.As(err, &ae) {
    fmt.Println(ae.Code, ae.Category, ae.Suggestion)
    fmt.Println(ae.Context) // e.g. {"session_id": "bad-id"}
}
```
