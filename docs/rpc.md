# RPC Reference

JSON-RPC 2.0 over WebSocket at `127.0.0.1:7600`.

## Methods

| Method | Params | Returns |
|--------|--------|---------|
| `ping` | none | `{status}` |
| `create` | `{command, args?, rows?, cols?}` | `{id}` |
| `screenshot` | `{id, format?, scrollback?}` | `{rows, cols, hash, lines?, history?, changes?, cursor, elements?, state?}` |
| `detect` | `{id, format?}` | `{elements}` or `{elements, tree, viewport, scrolled}` |
| `input` | `{id, data}` | `null` |
| `resize` | `{id, rows, cols}` | `null` |
| `mouse_click` | `{id, row, col, button?}` | `null` |
| `mouse_move` | `{id, row, col}` | `null` |
| `exec` | `{id, input, wait_text?, timeout_ms?}` | `{screen}` |
| `wait` | `{id, text?, stable?, gone?, regex?, timeout_ms?}` | `null` |
| `pipeline` | `{id, steps, stop_on_error?}` | `{results}` |
| `record` | `{id, path}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions}` |

## Screenshot Formats

The `format` parameter controls what the screenshot response includes:

| Format | Lines | Elements | State | Changes |
|--------|-------|----------|-------|---------|
| *(default)* | yes | no | no | no |
| `full` | yes | yes | yes | no |
| `structured` | no | yes | yes | no |
| `diff` | no | no | yes | yes (with `base_hash`) |

## Detect

`detect` supports two modes:

- default / `flat` — backward-compatible flat element list
- `structured` — richer semantic output intended for agents and higher-level tooling

Structured detect returns:

- `elements` — flattened semantic element list with `id`, `ref`, `role`, `description`, bounds, and state flags
- `tree` — hierarchical nesting of the same semantic elements
- `viewport` — current visible terminal rectangle
- `scrolled` — whether scroll indicators were detected

Example request:

```json
{"id":"sess-123","format":"structured"}
```

## Exec

Sends input followed by a carriage return, then waits for the screen to stabilize (or for `wait_text` to appear). Returns the screen state after completion.

## Wait

Unified wait method supporting multiple conditions:

- `text` — block until the text appears on screen
- `stable` — block until the screen stops changing (500ms threshold)
- `gone` — block until the text disappears from screen
- `regex` — block until a regex pattern matches screen content

Exactly one condition must be provided. Default timeout is 5000ms.

## Pipeline

Execute a sequence of steps against a single session. Each step has a `type` and type-specific fields:

| Step Type | Fields | Description |
|-----------|--------|-------------|
| `screenshot` | — | Capture current screen |
| `type` | `text` | Send literal text |
| `press` | `keys` | Send a named key (e.g. `Enter`, `Ctrl+C`) |
| `exec` | `input`, `timeout_ms?` | Send input + Enter, wait for stable |
| `wait` | `text?`, `stable?`, `gone?`, `regex?`, `timeout_ms?` | Wait for a condition |
| `sleep` | `ms` | Pause for N milliseconds |

Set `stop_on_error` to halt the pipeline on the first failing step.

## Authentication

Set `AWN_TOKEN` on both daemon and client to enable Bearer token auth. All WebSocket requests must include the token in the `Authorization` header.
