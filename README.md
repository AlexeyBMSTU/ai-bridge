# AI Bridge

A minimal, zero-dependency Go client for OpenAI-compatible chat completion APIs
(Groq, OpenAI, OpenRouter, vLLM, ...).

AI Bridge speaks the streaming (SSE) protocol under the hood and hands you back the
fully assembled assistant reply as a single string — no streaming plumbing on your side.

## Features

- **Zero dependencies** — Go standard library only
- **Any OpenAI-compatible endpoint** — just point it at a `/chat/completions` URL
- **Streaming made simple** — SSE chunks are consumed and concatenated for you
- **Truly asynchronous** — `Send` dispatches the request in the background and returns immediately
- **Channel-based API** — every `Send` call returns its own result channel
- **Concurrency-safe** — the shared conversation history is mutex-protected; responses never get mixed
- **Bounded concurrency** — a semaphore caps the number of in-flight requests
- **Interactive console mode** — built-in REPL via `StartConsoleDialog`
- **Context-aware** — deadlines and cancellation through `context.Context`

## Requirements

- Go 1.22 or newer

## Getting Started

```bash
export GROQ_API_KEY=your-key-here   # see note below
go run .
```

> **Note:** the example below reads the API key from the `GROQ_API_KEY` environment
> variable. Never hardcode API keys in source code.

## Usage

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

func main() {
	key := os.Getenv("GROQ_API_KEY")

	b := NewAiBridge(
		"https://api.groq.com/openai/v1/chat/completions",
		key,
		"openai/gpt-oss-120b",
		1, // countPools: max concurrent requests
		0, // timeout hint, 0 = default (3s)
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// One-shot request
	res := <-b.Send(ctx, "sup")
	if res.Err != nil {
		fmt.Println("error:", res.Err)
		return
	}
	fmt.Println(b.schema.Model, ">", res.Text)

	// Interactive console dialog
	b.StartConsoleDialog(ctx)
}
```

## API

### `NewAiBridge(url, key, modelName string, countPools int, timeDuration time.Duration) *BridgeStruct`

Creates a new bridge instance.

| Parameter       | Description                                                                    |
|-----------------|--------------------------------------------------------------------------------|
| `url`           | Full endpoint URL, e.g. `https://api.groq.com/openai/v1/chat/completions`      |
| `key`           | API key, sent as `Authorization: Bearer <key>`                                  |
| `modelName`     | Model identifier, e.g. `openai/gpt-oss-120b`                                    |
| `countPools`    | Max number of concurrent in-flight requests (semaphore capacity)                |
| `timeDuration`  | Per-request timeout hint; `0` selects the default (3s)                          |

> The effective deadline is governed by the `context.Context` you pass to `Send`.

Request parameters are fixed to the following defaults at construction time
(there are currently no setters to change them):

| Field                   | Default   |
|-------------------------|-----------|
| `reasoning_effort`      | `medium`  |
| `temperature`           | `1`       |
| `max_completion_tokens` | `2048`    |
| `top_p`                 | `1`       |
| `stream`                | `true`    |

### `Send(ctx context.Context, content string) <-chan BridgeResult`

Sends a user message asynchronously — it returns immediately, and the outcome
is delivered on a channel dedicated to this call (buffered, size 1), so
concurrent calls never mix responses.

What it does:

1. Spawns a background goroutine and returns the result channel right away.
2. The goroutine waits for a free pool slot — while `countPools` requests are
   in flight it queues, and reports `ctx.Err()` if the context is done first.
3. Once a slot is acquired, `content` is appended to the conversation history
   as a `user` message under a mutex, and a consistent snapshot is marshaled.
4. The request is POSTed, the SSE stream is read, all content deltas are
   concatenated, and the result is delivered on the channel.

`BridgeResult`:

| Field  | Description                         |
|--------|-------------------------------------|
| `Text` | The fully assembled assistant reply |
| `Err`  | Non-`nil` if the request failed     |

Read the reply from the returned channel:

```go
res := <-b.Send(ctx, "sup")
if res.Err != nil {
	// handle error
}
reply := res.Text
```

### `StartConsoleDialog(ctx context.Context)`

Runs an interactive read–send–print loop over stdin: every line is sent as a
user message, and the reply is printed as:

```
openai/gpt-oss-120b > Hello! How can I help you?
```

The loop ends on EOF (`Ctrl+D`), a scan error, or the first send error.

## Concurrency

`Send` is safe for concurrent use:

- each call gets its own result channel — responses are never mixed;
- the conversation history is guarded by a mutex, and every request carries a
  consistent snapshot that always includes its own message;
- `countPools` caps the number of in-flight HTTP requests — additional calls
  queue on the semaphore until a slot frees or their context is done.

Fan out several requests and collect the replies:

```go
chs := make([]<-chan BridgeResult, 3)
for i := range chs {
	chs[i] = b.Send(ctx, fmt.Sprintf("question %d", i))
}
for i, ch := range chs {
	res := <-ch
	if res.Err != nil {
		// handle error
		continue
	}
	fmt.Println(i, "->", res.Text)
}
```

> **Note:** all requests sent through one `BridgeStruct` share a single
> conversation history. Under concurrent dispatch, the order in which messages
> are appended is the order in which pool slots are acquired.

## Error Handling

All errors are reported through `BridgeResult.Err` on the returned channel:

| Situation                    | Error                                          |
|------------------------------|------------------------------------------------|
| Non-2xx HTTP status          | `unexpected status: <status>`                   |
| Malformed SSE payload        | `unmarshal chunk: ...` / `scan: ...`            |
| Context canceled / timed out | `context.Canceled` / `context.DeadlineExceeded` |
