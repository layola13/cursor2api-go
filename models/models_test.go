// Copyright (c) 2025-2026 libaxuan
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package models

import (
	"strings"
	"testing"
)

func TestGetStringContent(t *testing.T) {
	tests := []struct {
		name     string
		content  interface{}
		expected string
	}{
		{
			name:     "string content",
			content:  "Hello world",
			expected: "Hello world",
		},
		{
			name: "array content",
			content: []ContentPart{
				{Type: "text", Text: "Hello"},
				{Type: "text", Text: " world"},
			},
			expected: "Hello world",
		},
		{
			name:     "empty array",
			content:  []ContentPart{},
			expected: "",
		},
		{
			name:     "nil content",
			content:  nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{Content: tt.content}
			result := msg.GetStringContent()
			if result != tt.expected {
				t.Errorf("GetStringContent() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestToCursorMessages(t *testing.T) {
	tests := []struct {
		name             string
		messages         []Message
		systemPrompt     string
		expectedLength   int
		expectedFirstMsg string
	}{
		{
			name: "no system prompt",
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			systemPrompt:     "",
			expectedLength:   1,
			expectedFirstMsg: "Hello",
		},
		{
			name: "with system prompt, no system message",
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			systemPrompt:     "You are a helpful assistant",
			expectedLength:   2,
			expectedFirstMsg: "You are a helpful assistant",
		},
		{
			name: "with system prompt, has system message",
			messages: []Message{
				{Role: "system", Content: "Be helpful"},
				{Role: "user", Content: "Hello"},
			},
			systemPrompt:     "You are an AI",
			expectedLength:   2,
			expectedFirstMsg: "Be helpful\nYou are an AI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToCursorMessages(tt.messages, tt.systemPrompt)
			if len(result) != tt.expectedLength {
				t.Errorf("ToCursorMessages() length = %v, want %v", len(result), tt.expectedLength)
			}
			if len(result) > 0 && result[0].Parts[0].Text != tt.expectedFirstMsg {
				t.Errorf("ToCursorMessages() first message = %v, want %v", result[0].Parts[0].Text, tt.expectedFirstMsg)
			}
		})
	}
}

func TestNewChatCompletionResponse(t *testing.T) {
	response := NewChatCompletionResponse("test-id", "gpt-4o", "Hello world", Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})

	if response.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", response.ID)
	}
	if response.Model != "gpt-4o" {
		t.Errorf("Model = %v, want gpt-4o", response.Model)
	}
	if response.Choices[0].Message.Content != "Hello world" {
		t.Errorf("Content = %v, want Hello world", response.Choices[0].Message.Content)
	}
	if response.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %v, want 10", response.Usage.PromptTokens)
	}
}

func TestNewChatCompletionStreamResponse(t *testing.T) {
	response := NewChatCompletionStreamResponse("test-id", "gpt-4o", "Hello", stringPtr("stop"), nil)

	if response.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", response.ID)
	}
	if response.Choices[0].Delta.Content != "Hello" {
		t.Errorf("Content = %v, want Hello", response.Choices[0].Delta.Content)
	}
	if response.Choices[0].FinishReason == nil || *response.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %v, want stop", response.Choices[0].FinishReason)
	}
}

func TestNeedToolCallBridge(t *testing.T) {
	request := &ChatCompletionRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}
	if NeedToolCallBridge(request) {
		t.Errorf("NeedToolCallBridge() = true, want false")
	}

	request.Tools = []ToolSpec{
		{
			Type: "function",
			Function: FunctionSpec{
				Name: "read_file",
			},
		},
	}
	if !NeedToolCallBridge(request) {
		t.Errorf("NeedToolCallBridge() = false, want true")
	}
}

func TestBuildToolCallBridgePrompt(t *testing.T) {
	prompt := BuildToolCallBridgePrompt(
		[]ToolSpec{
			{
				Type: "function",
				Function: FunctionSpec{
					Name:        "read_file",
					Description: "Read file from path",
					Parameters:  []byte(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
				},
			},
		},
		nil,
		"required",
		nil,
	)

	if prompt == "" {
		t.Fatalf("BuildToolCallBridgePrompt() returned empty prompt")
	}

	if !strings.Contains(prompt, "<read_file>") {
		t.Errorf("prompt does not contain tool tag, got: %s", prompt)
	}
	if !strings.Contains(prompt, "<parameter1_name>value1</parameter1_name>") {
		t.Errorf("prompt does not contain Roo-style XML format guidance, got: %s", prompt)
	}
	if !strings.Contains(prompt, "parameters:") {
		t.Errorf("prompt does not contain parameters schema, got: %s", prompt)
	}
	if !strings.Contains(prompt, "required:") {
		t.Errorf("prompt does not contain required parameters hint, got: %s", prompt)
	}
	if !strings.Contains(prompt, "CRITICAL: The client requires a tool call now.") {
		t.Errorf("prompt does not contain forced tool call instruction, got: %s", prompt)
	}
}

func TestNormalizeMessagesForToolBridge(t *testing.T) {
	callID := "call_1"
	normalized := NormalizeMessagesForToolBridge([]Message{
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					Type: "function",
					Function: Function{
						Name:      "read_file",
						Arguments: "{\"path\":\"main.go\"}",
					},
				},
			},
		},
		{
			Role:       "tool",
			ToolCallID: &callID,
			Content:    "file content",
		},
	})

	if len(normalized) != 2 {
		t.Fatalf("len(normalized) = %d, want 2", len(normalized))
	}

	if normalized[0].GetStringContent() == "" {
		t.Errorf("assistant tool call not normalized into content")
	}
	if !strings.Contains(normalized[0].GetStringContent(), "<path>main.go</path>") {
		t.Errorf("assistant tool call is not converted to xml param tags: %s", normalized[0].GetStringContent())
	}

	if normalized[1].Role != "user" {
		t.Errorf("tool role normalized to %q, want user", normalized[1].Role)
	}
	if !strings.Contains(normalized[1].GetStringContent(), "<tool_result") {
		t.Errorf("tool result xml not found, got: %s", normalized[1].GetStringContent())
	}
	if !strings.Contains(normalized[1].GetStringContent(), "<![CDATA[file content]]>") {
		t.Errorf("tool result is not wrapped by cdata, got: %s", normalized[1].GetStringContent())
	}
}

func TestContainsXMLToolCall(t *testing.T) {
	tools := []ToolSpec{
		{
			Type: "function",
			Function: FunctionSpec{
				Name: "write_file",
			},
		},
	}

	if !ContainsXMLToolCall("<write_file><path>a.cpp</path></write_file>", tools, nil) {
		t.Errorf("ContainsXMLToolCall should detect direct tool tag")
	}
	if !ContainsXMLToolCall("<tool_calls>\n<write_file></write_file>\n</tool_calls>", tools, nil) {
		t.Errorf("ContainsXMLToolCall should detect tool_calls wrapper")
	}
	if ContainsXMLToolCall("I cannot use tools", tools, nil) {
		t.Errorf("ContainsXMLToolCall should be false for plain text")
	}
}

func TestExtractXMLToolCalls(t *testing.T) {
	tools := []ToolSpec{
		{
			Type: "function",
			Function: FunctionSpec{
				Name: "write_file",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name: "run_command",
			},
		},
	}

	content := `<tool_calls>
<write_file>
<path>/tmp/sort.cpp</path>
<content><![CDATA[#include <vector>]]></content>
</write_file>
<run_command>
<command>g++ /tmp/sort.cpp -o /tmp/sort</command>
</run_command>
</tool_calls>`

	calls, ok := ExtractXMLToolCalls(content, tools, nil)
	if !ok {
		t.Fatalf("ExtractXMLToolCalls() should detect tool calls")
	}
	if len(calls) != 2 {
		t.Fatalf("ExtractXMLToolCalls() len = %d, want 2", len(calls))
	}
	if calls[0].Name != "write_file" {
		t.Fatalf("first call name = %s, want write_file", calls[0].Name)
	}
	if !strings.Contains(calls[0].Arguments, `"path":"/tmp/sort.cpp"`) {
		t.Fatalf("first call args missing path, got: %s", calls[0].Arguments)
	}
	if !strings.Contains(calls[1].Arguments, `"command":"g++ /tmp/sort.cpp -o /tmp/sort"`) {
		t.Fatalf("second call args missing command, got: %s", calls[1].Arguments)
	}
}

func TestExtractNonToolTextFromXMLContent(t *testing.T) {
	tools := []ToolSpec{
		{
			Type: "function",
			Function: FunctionSpec{
				Name: "write_file",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name: "run_command",
			},
		},
	}

	content := `先写文件
<tool_calls>
<write_file>
<path>sort.cpp</path>
<content>abc</content>
</write_file>
<run_command>
<command>g++ sort.cpp</command>
</run_command>
</tool_calls>
最后说明`

	text := ExtractNonToolTextFromXMLContent(content, tools, nil)
	if !strings.Contains(text, "先写文件") {
		t.Fatalf("non-tool text should keep leading explanation, got: %s", text)
	}
	if !strings.Contains(text, "最后说明") {
		t.Fatalf("non-tool text should keep trailing explanation, got: %s", text)
	}
	if strings.Contains(text, "<write_file>") || strings.Contains(text, "<run_command>") {
		t.Fatalf("non-tool text should not include xml tool blocks, got: %s", text)
	}
}

func TestBuildNoToolsUsedRetryPrompt(t *testing.T) {
	prompt := BuildNoToolsUsedRetryPrompt(
		[]ToolSpec{
			{
				Type: "function",
				Function: FunctionSpec{
					Name: "run_command",
				},
			},
		},
		nil,
	)

	if !strings.Contains(prompt, "[ERROR] You did not use a tool") {
		t.Errorf("retry prompt missing error heading: %s", prompt)
	}
	if !strings.Contains(prompt, "<run_command>") {
		t.Errorf("retry prompt missing tool list: %s", prompt)
	}
}

func TestNewChatCompletionToolCallResponse(t *testing.T) {
	response := NewChatCompletionToolCallResponse(
		"test-id",
		"gpt-4o",
		[]ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: Function{
					Name:      "write_file",
					Arguments: `{"path":"a.cpp"}`,
				},
			},
		},
		"tool call summary",
		Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	)

	if response.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("FinishReason = %v, want tool_calls", response.Choices[0].FinishReason)
	}
	if len(response.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls len = %d, want 1", len(response.Choices[0].Message.ToolCalls))
	}
	if response.Choices[0].Message.ToolCalls[0].Function.Name != "write_file" {
		t.Fatalf("tool call name = %s, want write_file", response.Choices[0].Message.ToolCalls[0].Function.Name)
	}
	if response.Choices[0].Message.Content != "tool call summary" {
		t.Fatalf("tool call response content should be preserved, got %v", response.Choices[0].Message.Content)
	}
}

func TestNewErrorResponse(t *testing.T) {
	response := NewErrorResponse("Test error", "test_error", "error_code")

	if response.Error.Message != "Test error" {
		t.Errorf("Message = %v, want Test error", response.Error.Message)
	}
	if response.Error.Type != "test_error" {
		t.Errorf("Type = %v, want test_error", response.Error.Type)
	}
	if response.Error.Code != "error_code" {
		t.Errorf("Code = %v, want error_code", response.Error.Code)
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
