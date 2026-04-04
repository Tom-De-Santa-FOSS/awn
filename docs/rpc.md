# RPC Reference

JSON-RPC 2.0 over WebSocket at `127.0.0.1:7600`.

## Methods

| Method | Params | Returns |
|--------|--------|---------|
| `ping` | none | `{status}` |
| `create` | `{command, args?, rows?, cols?, scrollback?}` | `{id}` |
| `screenshot` | `{id, format?, scrollback?}` | `{rows, cols, hash, lines?, history?, changes?, cursor, elements?, state?}` |
| `detect` | `{id}` | `{elements}` |
| `input` | `{id, data}` | `null` |
| `resize` | `{id, rows, cols}` | `null` |
| `mouse_click` | `{id, row, col, button?}` | `null` |
| `mouse_move` | `{id, row, col}` | `null` |
| `wait_for_text` | `{id, text, timeout_ms?}` | `null` |
| `wait_for_stable` | `{id, stable_ms?, timeout_ms?}` | `null` |
| `record` | `{id, path}` | `null` |
| `close` | `{id}` | `null` |
| `list` | none | `{sessions}` |

## Authentication

Set `AWN_TOKEN` on both daemon and client to enable Bearer token auth. All WebSocket requests must include the token in the `Authorization` header.

## Subscriptions

Use `subscribe` and `unsubscribe` over the WebSocket connection to receive real-time screen update notifications for a session.
