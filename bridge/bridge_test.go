package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newSSEServer returns a server that streams a single SSE chunk echoing the
// last message of the request back as the assistant reply.
func newSSEServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SchemaAPI
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		reply := req.Messages[len(req.Messages)-1].Content

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n", reply)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

// Concurrent Send calls must each receive their own reply on their own
// channel. Run with -race to verify the shared history is protected.
func TestSendConcurrent(t *testing.T) {
	srv := newSSEServer(t)
	defer srv.Close()

	b := NewAiBridge(srv.URL, "test-key", "test-model", 2, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const n = 8
	chs := make([]<-chan BridgeResult, n)
	for i := 0; i < n; i++ {
		chs[i] = b.Send(ctx, fmt.Sprintf("msg-%d", i))
	}
	for i, ch := range chs {
		res := <-ch
		if res.Err != nil {
			t.Fatalf("send %d: %v", i, res.Err)
		}
		want := fmt.Sprintf("msg-%d", i)
		if res.Text != want {
			t.Fatalf("send %d: got %q, want %q", i, res.Text, want)
		}
	}
}
