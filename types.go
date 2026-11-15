package bridge

import (
	"net/http"
	"sync"
	"time"
)

type RoleSchema struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type SchemaAPI struct {
	Messages            []RoleSchema `json:"messages"`
	Model               string       `json:"model"`
	ReasoningEffort     string       `json:"reasoning_effort"`
	Temperature         float64      `json:"temperature"`
	MaxCompletionTokens int          `json:"max_completion_tokens"`
	TopP                float64      `json:"top_p"`
	Stream              bool         `json:"stream"`
	Stop                *string      `json:"stop"`
}

type ChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type ChunkChoice struct {
	Index        int        `json:"index"`
	Delta        ChunkDelta `json:"delta"`
	FinishReason *string    `json:"finish_reason"`
}

type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
}

type BridgeResult struct {
	Text string
	Err  error
}

type BridgeStruct struct {
	schema     SchemaAPI
	url        string
	key        string
	countPools int
	client     http.Client
	timeout    time.Duration
	mu         sync.Mutex
	sem        chan struct{}
}

func NewAiBridge(url, key, modelName string, countPools int, timeDuration time.Duration) BridgeStruct {
	schema := SchemaAPI{
		Messages:            []RoleSchema{},
		Model:               modelName,
		ReasoningEffort:     "medium",
		Temperature:         1,
		MaxCompletionTokens: 2048,
		TopP:                1,
		Stream:              true,
		Stop:                nil,
	}
	if timeDuration == 0 {
		timeDuration = 3 * time.Second
	}

	return BridgeStruct{
		schema:     schema,
		url:        url,
		key:        key,
		countPools: countPools,
		client:     http.Client{Timeout: 0},
		timeout:    timeDuration,
		sem:        make(chan struct{}, countPools),
	}
}
