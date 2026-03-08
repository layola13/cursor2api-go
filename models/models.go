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
	"encoding/json"
	"strings"
	"time"
)

// ChatCompletionRequest OpenAI聊天完成请求
type ChatCompletionRequest struct {
	Model             string         `json:"model" binding:"required"`
	Messages          []Message      `json:"messages" binding:"required"`
	Stream            bool           `json:"stream,omitempty"`
	Temperature       *float64       `json:"temperature,omitempty"`
	MaxTokens         *int           `json:"max_tokens,omitempty"`
	TopP              *float64       `json:"top_p,omitempty"`
	Stop              []string       `json:"stop,omitempty"`
	User              string         `json:"user,omitempty"`
	Tools             []ToolSpec     `json:"tools,omitempty"`
	Functions         []FunctionSpec `json:"functions,omitempty"`
	ToolChoice        interface{}    `json:"tool_choice,omitempty"`
	FunctionCall      interface{}    `json:"function_call,omitempty"`
	ParallelToolCalls *bool          `json:"parallel_tool_calls,omitempty"`
}

// Message 消息结构
type Message struct {
	Role         string        `json:"role" binding:"required"`
	Content      interface{}   `json:"content,omitempty"`
	Images       []string      `json:"images,omitempty"`
	Attachments  []interface{} `json:"attachments,omitempty"`
	Files        []interface{} `json:"files,omitempty"`
	ImageURL     interface{}   `json:"image_url,omitempty"`
	InputImage   interface{}   `json:"input_image,omitempty"`
	ToolCallID   *string       `json:"tool_call_id,omitempty"`
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
	FunctionCall *Function     `json:"function_call,omitempty"`
}

// ToolCall 工具调用结构
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function 函数调用结构
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolSpec 工具定义结构
type ToolSpec struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

// FunctionSpec 函数定义结构
type FunctionSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ContentPart 消息内容部分（用于多模态内容）
type ContentPart struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	URL       string                 `json:"url,omitempty"`
	ImageURL  map[string]interface{} `json:"image_url,omitempty"`
	Image     string                 `json:"image,omitempty"`
	MediaType string                 `json:"mediaType,omitempty"`
	Source    map[string]interface{} `json:"source,omitempty"`
}

// ChatCompletionResponse OpenAI聊天完成响应
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// ChatCompletionStreamResponse 流式响应
type ChatCompletionStreamResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// Choice 选择结构
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// StreamChoice 流式选择结构
type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        StreamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// StreamDelta 流式增量数据
type StreamDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []StreamToolCall `json:"tool_calls,omitempty"`
}

// StreamToolCall 流式 tool_calls 增量结构
type StreamToolCall struct {
	Index    int                  `json:"index"`
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"`
	Function *StreamFunctionDelta `json:"function,omitempty"`
}

// StreamFunctionDelta 流式 function 增量结构
type StreamFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// Usage 使用统计
type Usage struct {
	PromptTokens            int                     `json:"prompt_tokens"`
	CompletionTokens        int                     `json:"completion_tokens"`
	TotalTokens             int                     `json:"total_tokens"`
	PromptTokensDetails     *PromptTokensDetails    `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *CompletionTokensDetail `json:"completion_tokens_details,omitempty"`
}

// PromptTokensDetails 输入 token 明细
type PromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
}

// CompletionTokensDetail 输出 token 明细
type CompletionTokensDetail struct {
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AudioTokens              int `json:"audio_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

// Model 模型信息
type Model struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Created       int64  `json:"created"`
	OwnedBy       string `json:"owned_by"`
	MaxTokens     int    `json:"max_tokens,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`
}

// ModelsResponse 模型列表响应
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// CursorMessage Cursor消息格式
type CursorMessage struct {
	Role  string       `json:"role"`
	Parts []CursorPart `json:"parts"`
}

// CursorImageURL Cursor图片URL结构
type CursorImageURL struct {
	URL string `json:"url"`
}

// CursorImageSource Cursor图片base64数据结构（Anthropic风格）
type CursorImageSource struct {
	Type          string `json:"type"`
	MediaType     string `json:"media_type,omitempty"`
	MediaTypeAlt  string `json:"mediaType,omitempty"`
	Data          string `json:"data"`
}

// CursorPart Cursor消息部分
type CursorPart struct {
	Type      string             `json:"type"`
	Text      string             `json:"text,omitempty"`
	ImageURL  *CursorImageURL    `json:"image_url,omitempty"`
	Source    *CursorImageSource `json:"source,omitempty"`
	Image     string             `json:"image,omitempty"`
	MediaType string             `json:"mediaType,omitempty"`
}

// CursorRequest Cursor请求格式
type CursorRequest struct {
	Context  []interface{}   `json:"context"`
	Model    string          `json:"model"`
	ID       string          `json:"id"`
	Messages []CursorMessage `json:"messages"`
	Trigger  string          `json:"trigger"`
}

// CursorEventData Cursor事件数据
type CursorEventData struct {
	Type            string                 `json:"type"`
	Delta           string                 `json:"delta,omitempty"`
	ErrorText       string                 `json:"errorText,omitempty"`
	MessageMetadata *CursorMessageMetadata `json:"messageMetadata,omitempty"`
}

// CursorMessageMetadata Cursor消息元数据
type CursorMessageMetadata struct {
	Usage *CursorUsage `json:"usage,omitempty"`
}

// CursorUsage Cursor使用统计
type CursorUsage struct {
	InputTokens              int `json:"inputTokens"`
	OutputTokens             int `json:"outputTokens"`
	TotalTokens              int `json:"totalTokens"`
	CachedInputTokens        int `json:"cachedInputTokens,omitempty"`
	AudioInputTokens         int `json:"audioInputTokens,omitempty"`
	ReasoningOutputTokens    int `json:"reasoningOutputTokens,omitempty"`
	AudioOutputTokens        int `json:"audioOutputTokens,omitempty"`
	AcceptedPredictionTokens int `json:"acceptedPredictionTokens,omitempty"`
	RejectedPredictionTokens int `json:"rejectedPredictionTokens,omitempty"`
}

// SSEEvent 服务器发送事件
type SSEEvent struct {
	Data  string `json:"data"`
	Event string `json:"event,omitempty"`
	ID    string `json:"id,omitempty"`
}

// GetStringContent 获取消息的字符串内容
func (m *Message) GetStringContent() string {
	if m.Content == nil {
		return ""
	}

	switch content := m.Content.(type) {
	case string:
		return content
	case []ContentPart:
		var text string
		for _, part := range content {
			if part.Type == "text" {
				text += part.Text
			}
		}
		return text
	case []interface{}:
		// 处理混合类型内容
		var text string
		for _, item := range content {
			if part, ok := item.(map[string]interface{}); ok {
				if partType, exists := part["type"].(string); exists && (partType == "text" || partType == "input_text") {
					text += strings.TrimSpace(toString(part["text"]))
				}
			}
		}
		return text
	default:
		// 尝试将其他类型转换为JSON字符串
		if data, err := json.Marshal(content); err == nil {
			return string(data)
		}
		return ""
	}
}

// ToCursorMessages 将OpenAI消息转换为Cursor格式
func ToCursorMessages(messages []Message, systemPromptInject string) []CursorMessage {
	var result []CursorMessage

	// 处理系统提示注入
	if systemPromptInject != "" {
		if len(messages) > 0 && messages[0].Role == "system" {
			// 如果第一条已经是系统消息，追加注入内容
			content := strings.TrimSpace(messages[0].GetStringContent())
			if content == "" {
				content = systemPromptInject
			} else {
				content += "\n" + systemPromptInject
			}
			result = append(result, CursorMessage{
				Role: "system",
				Parts: []CursorPart{
					{Type: "text", Text: content},
				},
			})
			messages = messages[1:] // 跳过第一条消息
		} else {
			// 如果第一条不是系统消息或没有消息，插入新的系统消息
			result = append(result, CursorMessage{
				Role: "system",
				Parts: []CursorPart{
					{Type: "text", Text: systemPromptInject},
				},
			})
		}
	} else if len(messages) > 0 && messages[0].Role == "system" {
		// 如果有系统消息但没有注入内容，直接添加
		result = append(result, CursorMessage{
			Role: "system",
			Parts: []CursorPart{
				{Type: "text", Text: messages[0].GetStringContent()},
			},
		})
		messages = messages[1:] // 跳过第一条消息
	}

	// 转换其余消息
	for _, msg := range messages {
		if msg.Role == "" {
			continue // 跳过空消息
		}

		cursorMsg := CursorMessage{
			Role:  msg.Role,
			Parts: messageToCursorParts(msg),
		}
		result = append(result, cursorMsg)
	}

	return result
}

// HasImageContent 判断消息列表中是否包含图片输入
func HasImageContent(messages []Message) bool {
	for _, msg := range messages {
		if messageHasImage(msg) {
			return true
		}
	}
	return false
}

func messageHasImage(msg Message) bool {
	if len(collectMessageLevelImageURLs(msg)) > 0 {
		return true
	}

	if msg.Content == nil {
		return false
	}

	switch content := msg.Content.(type) {
	case []ContentPart:
		for _, part := range content {
			partType := strings.ToLower(strings.TrimSpace(part.Type))
			if partType == "image_url" || partType == "input_image" || partType == "image" {
				return true
			}
			if strings.TrimSpace(part.URL) != "" {
				return true
			}
			if len(part.ImageURL) > 0 {
				return true
			}
			if strings.TrimSpace(part.Image) != "" {
				return true
			}
			if len(part.Source) > 0 {
				return true
			}
		}
	case []interface{}:
		for _, item := range content {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			partType := strings.ToLower(strings.TrimSpace(toString(part["type"])))
			if partType == "image_url" || partType == "input_image" || partType == "image" {
				return true
			}
			if _, ok := part["image_url"]; ok {
				return true
			}
			if _, ok := part["source"]; ok {
				return true
			}
			if _, ok := part["image"]; ok {
				return true
			}
			if _, ok := part["input_image"]; ok {
				return true
			}
		}
	}

	return false
}

func messageToCursorParts(msg Message) []CursorPart {
	parts := make([]CursorPart, 0, 4)

	switch content := msg.Content.(type) {
	case string:
		if strings.TrimSpace(content) != "" {
			parts = append(parts, CursorPart{Type: "text", Text: content})
		}

	case []ContentPart:
		for _, part := range content {
			switch strings.ToLower(strings.TrimSpace(part.Type)) {
			case "text", "input_text":
				if strings.TrimSpace(part.Text) != "" {
					parts = append(parts, CursorPart{Type: "text", Text: part.Text})
				}
			case "image_url", "input_image", "image":
				if url := extractImageURLFromContentPart(part); url != "" {
					parts = append(parts, CursorPart{
						Type:     "image_url",
						ImageURL: &CursorImageURL{URL: url},
					})
				}
			}
		}

	case []interface{}:
		for _, item := range content {
			partMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			partType := strings.ToLower(strings.TrimSpace(toString(partMap["type"])))
			switch partType {
			case "text", "input_text":
				text := strings.TrimSpace(toString(partMap["text"]))
				if text != "" {
					parts = append(parts, CursorPart{Type: "text", Text: text})
				}
			case "image_url", "input_image", "image":
				if url := extractImageURL(partMap); url != "" {
					parts = append(parts, CursorPart{
						Type:     "image_url",
						ImageURL: &CursorImageURL{URL: url},
					})
				}
			default:
				if url := extractImageURL(partMap); url != "" {
					parts = append(parts, CursorPart{
						Type:     "image_url",
						ImageURL: &CursorImageURL{URL: url},
					})
				}
			}
		}
	}

	parts = appendImageParts(parts, collectMessageLevelImageURLs(msg))
	if len(parts) > 0 {
		return parts
	}

	fallback := msg.GetStringContent()
	if fallback != "" {
		return []CursorPart{{Type: "text", Text: fallback}}
	}
	return []CursorPart{{Type: "text", Text: ""}}
}

func extractImageURLFromContentPart(part ContentPart) string {
	if raw := strings.TrimSpace(part.URL); raw != "" {
		return raw
	}
	if len(part.ImageURL) > 0 {
		if url := extractURLField(part.ImageURL); url != "" {
			return url
		}
	}
	if strings.TrimSpace(part.Image) != "" {
		imageValue := strings.TrimSpace(part.Image)
		lower := strings.ToLower(imageValue)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:image/") {
			return imageValue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(part.MediaType)), "image/") {
			return "data:" + strings.TrimSpace(part.MediaType) + ";base64," + imageValue
		}
	}
	if len(part.Source) > 0 {
		if url := extractImageURL(part.Source); url != "" {
			return url
		}
	}
	return ""
}

func extractImageURL(partMap map[string]interface{}) string {
	// OpenAI-compatible: {"type":"image_url","image_url":{"url":"data:image/..."}}
	if raw, ok := partMap["image_url"]; ok {
		if url := extractURLField(raw); url != "" {
			return url
		}
	}
	if raw, ok := partMap["input_image"]; ok {
		if url := extractURLField(raw); url != "" {
			return url
		}
	}

	// Some clients send {"type":"image_url","url":"..."}
	if raw, ok := partMap["url"]; ok {
		if url := strings.TrimSpace(toString(raw)); url != "" {
			return url
		}
	}
	if raw, ok := partMap["href"]; ok {
		if url := strings.TrimSpace(toString(raw)); url != "" {
			return url
		}
	}

	// AI SDK-like image part: {"type":"image","image":"...","mediaType":"image/png"}
	if raw, ok := partMap["image"]; ok {
		imageValue := strings.TrimSpace(toString(raw))
		if imageValue != "" {
			lower := strings.ToLower(imageValue)
			if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:image/") {
				return imageValue
			}
			mediaType := extractMediaType(partMap)
			if strings.HasPrefix(strings.ToLower(mediaType), "image/") {
				return "data:" + mediaType + ";base64," + imageValue
			}
		}
	}

	// Anthropic-like image source: {"type":"image","source":{"type":"base64","media_type":"image/png","data":"..."}}
	if rawSource, ok := partMap["source"]; ok {
		if source, ok := rawSource.(map[string]interface{}); ok {
			mediaType := strings.TrimSpace(toString(source["media_type"]))
			if mediaType == "" {
				mediaType = strings.TrimSpace(toString(source["mediaType"]))
			}
			data := strings.TrimSpace(toString(source["data"]))
			if mediaType != "" && data != "" {
				return "data:" + mediaType + ";base64," + data
			}
		}
	}

	return ""
}

func extractMediaType(partMap map[string]interface{}) string {
	for _, key := range []string{"media_type", "mediaType", "mime_type", "mimeType"} {
		if raw, ok := partMap[key]; ok {
			if v := strings.TrimSpace(toString(raw)); v != "" {
				return v
			}
		}
	}
	return ""
}

func extractURLField(raw interface{}) string {
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		if url, ok := typed["url"]; ok {
			return strings.TrimSpace(toString(url))
		}
	case ContentPart:
		return strings.TrimSpace(typed.URL)
	case *ContentPart:
		if typed != nil {
			return strings.TrimSpace(typed.URL)
		}
	}
	return ""
}

func collectMessageLevelImageURLs(msg Message) []string {
	urls := make([]string, 0, len(msg.Images)+4)

	for _, raw := range msg.Images {
		if parsed := strings.TrimSpace(raw); parsed != "" {
			urls = append(urls, parsed)
		}
	}
	if parsed := extractURLField(msg.ImageURL); parsed != "" {
		urls = append(urls, parsed)
	}
	if parsed := extractURLField(msg.InputImage); parsed != "" {
		urls = append(urls, parsed)
	}

	for _, raw := range msg.Attachments {
		if parsed := extractImageURLFromAny(raw); parsed != "" {
			urls = append(urls, parsed)
		}
	}
	for _, raw := range msg.Files {
		if parsed := extractImageURLFromAny(raw); parsed != "" {
			urls = append(urls, parsed)
		}
	}

	return uniqueNonEmptyStrings(urls)
}

func extractImageURLFromAny(raw interface{}) string {
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		if url := extractURLField(typed["image_url"]); url != "" {
			return url
		}
		if url := extractURLField(typed["input_image"]); url != "" {
			return url
		}
		if url := extractURLField(typed["url"]); url != "" {
			return url
		}
		if url := extractURLField(typed["href"]); url != "" {
			return url
		}
		if dataURL := extractImageURL(typed); dataURL != "" {
			return dataURL
		}
	}
	return ""
}

func appendImageParts(parts []CursorPart, urls []string) []CursorPart {
	if len(urls) == 0 {
		return parts
	}
	seen := make(map[string]struct{}, len(parts)+len(urls))
	for _, part := range parts {
		if part.ImageURL == nil {
			continue
		}
		key := strings.TrimSpace(part.ImageURL.URL)
		if key != "" {
			seen[key] = struct{}{}
		}
	}
	for _, raw := range urls {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		parts = append(parts, CursorPart{
			Type:     "image_url",
			ImageURL: &CursorImageURL{URL: key},
		})
	}
	return parts
}

func uniqueNonEmptyStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func toString(v interface{}) string {
	switch typed := v.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		if typed == nil {
			return ""
		}
		data, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

// NewChatCompletionResponse 创建聊天完成响应
func NewChatCompletionResponse(id, model, content string, usage Usage) *ChatCompletionResponse {
	return &ChatCompletionResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: usage,
	}
}

// NewChatCompletionToolCallResponse 创建包含 tool_calls 的聊天完成响应
func NewChatCompletionToolCallResponse(id, model string, toolCalls []ToolCall, content string, usage Usage) *ChatCompletionResponse {
	message := Message{
		Role:      "assistant",
		ToolCalls: toolCalls,
	}
	if strings.TrimSpace(content) != "" {
		message.Content = content
	}

	return &ChatCompletionResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: "tool_calls",
			},
		},
		Usage: usage,
	}
}

// NewChatCompletionStreamResponse 创建流式响应
func NewChatCompletionStreamResponse(id, model, content string, finishReason *string, usage *Usage) *ChatCompletionStreamResponse {
	return &ChatCompletionStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []StreamChoice{
			{
				Index: 0,
				Delta: StreamDelta{
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(message, errorType, code string) *ErrorResponse {
	return &ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    errorType,
			Code:    code,
		},
	}
}
