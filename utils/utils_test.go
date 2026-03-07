package utils

import (
	"context"
	"cursor2api-go/models"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStreamChatCompletionIncludesUsageInFinishChunk(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := make(chan interface{}, 2)
	stream <- "hello"
	stream <- models.Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
	}
	close(stream)

	StreamChatCompletion(c, stream, "test-model")

	body := recorder.Body.String()
	if !strings.Contains(body, "\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}") {
		t.Fatalf("finish chunk does not include usage details, body=%s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("stream does not include DONE marker, body=%s", body)
	}
}

func TestReadSSEStreamParsesUsageDetails(t *testing.T) {
	sseBody := strings.Join([]string{
		`data: {"type":"delta","delta":"hello"}`,
		"",
		`data: {"type":"finish","messageMetadata":{"usage":{"inputTokens":12,"outputTokens":8,"totalTokens":20,"cachedInputTokens":4,"reasoningOutputTokens":3}}}`,
		"",
	}, "\n")

	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(sseBody)),
	}
	output := make(chan interface{}, 4)

	if err := ReadSSEStream(context.Background(), resp, output); err != nil {
		t.Fatalf("ReadSSEStream() error = %v", err)
	}

	close(output)

	var gotDelta string
	var gotUsage models.Usage
	for item := range output {
		switch v := item.(type) {
		case string:
			gotDelta = v
		case models.Usage:
			gotUsage = v
		}
	}

	if gotDelta != "hello" {
		t.Fatalf("delta = %q, want hello", gotDelta)
	}
	if gotUsage.TotalTokens != 20 {
		t.Fatalf("total_tokens = %d, want 20", gotUsage.TotalTokens)
	}
	if gotUsage.PromptTokensDetails == nil || gotUsage.PromptTokensDetails.CachedTokens != 4 {
		t.Fatalf("prompt_tokens_details.cached_tokens not parsed: %#v", gotUsage.PromptTokensDetails)
	}
	if gotUsage.CompletionTokensDetails == nil || gotUsage.CompletionTokensDetails.ReasoningTokens != 3 {
		t.Fatalf("completion_tokens_details.reasoning_tokens not parsed: %#v", gotUsage.CompletionTokensDetails)
	}
}
