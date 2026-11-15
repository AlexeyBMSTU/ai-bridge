package bridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func (b *BridgeStruct) fetch(ctx context.Context, content string) (string, error) {
	b.mu.Lock()
	b.schema.Messages = append(b.schema.Messages, RoleSchema{Role: "user", Content: content})
	raw, err := json.Marshal(b.schema)
	b.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.url, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.key)

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", fmt.Errorf("unmarshal chunk: %w", err)
		}
		for _, ch := range chunk.Choices {
			sb.WriteString(ch.Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan: %w", err)
	}

	return sb.String(), nil
}

// Send dispatches content in the background and returns a per-call channel
// with the result. A pool slot is acquired inside the goroutine, so calls
// beyond countPools queue until a slot frees or ctx is done.
func (b *BridgeStruct) Send(ctx context.Context, content string) <-chan BridgeResult {
	ch := make(chan BridgeResult, 1)
	go func() {
		select {
		case b.sem <- struct{}{}:
			defer func() { <-b.sem }()
			text, err := b.fetch(ctx, content)
			ch <- BridgeResult{Text: text, Err: err}
		case <-ctx.Done():
			ch <- BridgeResult{Err: ctx.Err()}
		}
	}()
	return ch
}

func (b *BridgeStruct) StartConsoleDialog(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		res := <-b.Send(ctx, scanner.Text())
		if res.Err != nil {
			fmt.Println("error:", res.Err)
			return
		}
		fmt.Println(b.schema.Model, ">", res.Text)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

