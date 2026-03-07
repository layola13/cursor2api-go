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

package handlers

import (
	"cursor2api-go/config"
	"cursor2api-go/middleware"
	"cursor2api-go/models"
	"cursor2api-go/services"
	"cursor2api-go/utils"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Handler 处理器结构
type Handler struct {
	config        *config.Config
	cursorService *services.CursorService
	docsContent   []byte
}

// NewHandler 创建新的处理器
func NewHandler(cfg *config.Config) *Handler {
	cursorService := services.NewCursorService(cfg)

	// 预加载文档内容
	docsPath := "static/docs.html"
	var docsContent []byte

	if data, err := os.ReadFile(docsPath); err == nil {
		docsContent = data
	} else {
		// 如果文件不存在，使用默认的简单HTML页面
		simpleHTML := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Cursor2API - Go Version</title>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background: white;
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            border-bottom: 2px solid #007bff;
            padding-bottom: 10px;
        }
        .info {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            margin: 20px 0;
            border-left: 4px solid #007bff;
        }
        code {
            background: #e9ecef;
            padding: 2px 6px;
            border-radius: 4px;
            font-family: 'Courier New', monospace;
        }
        .endpoint {
            background: #e3f2fd;
            padding: 10px;
            margin: 10px 0;
            border-radius: 5px;
            border-left: 3px solid #2196f3;
        }
        .status-ok {
            color: #28a745;
            font-weight: bold;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Cursor2API - Go Version</h1>
        
        <div class="info">
            <p><strong>Status:</strong> <span class="status-ok">✅ Running</span></p>
            <p><strong>Version:</strong> Go Implementation</p>
            <p><strong>Description:</strong> OpenAI-compatible API proxy for Cursor AI</p>
        </div>
        
        <div class="info">
            <h3>📡 Available Endpoints:</h3>
            <div class="endpoint">
                <strong>GET</strong> <code>/v1/models</code><br>
                <small>List available AI models</small>
            </div>
            <div class="endpoint">
                <strong>POST</strong> <code>/v1/chat/completions</code><br>
                <small>Create chat completion (supports streaming)</small>
            </div>
            <div class="endpoint">
                <strong>GET</strong> <code>/health</code><br>
                <small>Health check endpoint</small>
            </div>
        </div>
        
        <div class="info">
            <h3>🔐 Authentication:</h3>
            <p>Use Bearer token authentication:</p>
            <code>Authorization: Bearer YOUR_API_KEY</code>
            <p><small>Default API key: <code>0000</code> (change via API_KEY environment variable)</small></p>
        </div>
        
        <div class="info">
            <h3>💻 Example Usage:</h3>
            <pre><code>curl -X POST http://localhost:8002/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 0000" \
  -d '{
    "model": "claude-sonnet-4.6",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'</code></pre>
        </div>
        
        <div class="info">
            <p><strong>Repository:</strong> <a href="https://github.com/cursor2api/cursor2api-go">cursor2api-go</a></p>
            <p><strong>Documentation:</strong> OpenAI API compatible</p>
        </div>
    </div>
</body>
</html>`
		docsContent = []byte(simpleHTML)
	}

	return &Handler{
		config:        cfg,
		cursorService: cursorService,
		docsContent:   docsContent,
	}

}

// ListModels 列出可用模型
func (h *Handler) ListModels(c *gin.Context) {
	modelNames := h.config.GetModels()
	modelList := make([]models.Model, 0, len(modelNames))

	for _, modelID := range modelNames {
		// 获取模型配置信息
		modelConfig, exists := models.GetModelConfig(modelID)

		model := models.Model{
			ID:      modelID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "cursor2api",
		}

		// 如果找到模型配置，添加max_tokens和context_window信息
		if exists {
			model.MaxTokens = modelConfig.MaxTokens
			model.ContextWindow = modelConfig.ContextWindow
		}

		modelList = append(modelList, model)
	}

	response := models.ModelsResponse{
		Object: "list",
		Data:   modelList,
	}

	c.JSON(http.StatusOK, response)
}

// ChatCompletions 处理聊天完成请求
func (h *Handler) ChatCompletions(c *gin.Context) {
	var request models.ChatCompletionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logrus.WithError(err).Error("Failed to bind request")
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			"Invalid request format",
			"invalid_request_error",
			"invalid_json",
		))
		return
	}

	// 验证模型
	if !h.config.IsValidModel(request.Model) {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			"Invalid model specified",
			"invalid_request_error",
			"model_not_found",
		))
		return
	}

	// 验证消息
	if len(request.Messages) == 0 {
		c.JSON(http.StatusBadRequest, models.NewErrorResponse(
			"Messages cannot be empty",
			"invalid_request_error",
			"missing_messages",
		))
		return
	}

	// 验证并调整max_tokens参数
	request.MaxTokens = models.ValidateMaxTokens(request.Model, request.MaxTokens)

	// 非流式 + tool bridge: 如果模型未按 XML 输出工具调用，自动追加纠正提示并重试。
	if !request.Stream && models.NeedToolCallBridge(&request) {
		if handled := h.handleNonStreamToolBridgeWithRetry(c, &request); handled {
			return
		}
	}
	// 流式 + tool bridge: 首轮失败不向客户端输出，先在服务端缓存并重试，成功后再返回。
	if request.Stream && models.NeedToolCallBridge(&request) {
		if handled := h.handleStreamToolBridgeWithRetry(c, &request); handled {
			return
		}
	}

	// 调用Cursor服务
	chatGenerator, err := h.cursorService.ChatCompletion(c.Request.Context(), &request)
	if err != nil {
		logrus.WithError(err).Error("Failed to create chat completion")
		middleware.HandleError(c, err)
		return
	}

	// 根据是否流式返回不同响应
	if request.Stream {
		utils.SafeStreamWrapper(utils.StreamChatCompletion, c, chatGenerator, request.Model)
	} else {
		utils.NonStreamChatCompletion(c, chatGenerator, request.Model)
	}
}

func (h *Handler) handleNonStreamToolBridgeWithRetry(c *gin.Context, request *models.ChatCompletionRequest) bool {
	const maxToolBridgeRetries = 2

	messages := append([]models.Message(nil), request.Messages...)
	var lastUsage models.Usage
	var lastContent string

	for attempt := 0; attempt <= maxToolBridgeRetries; attempt++ {
		attemptReq := *request
		attemptReq.Messages = append([]models.Message(nil), messages...)
		attemptReq.Stream = false

		chatGenerator, err := h.cursorService.ChatCompletion(c.Request.Context(), &attemptReq)
		if err != nil {
			logrus.WithError(err).Error("Failed to create chat completion during tool bridge retry")
			middleware.HandleError(c, err)
			return true
		}

		content, usage, err := collectNonStreamResult(c, chatGenerator)
		if err != nil {
			middleware.HandleError(c, err)
			return true
		}

		lastContent = content
		lastUsage = usage

		if models.ContainsXMLToolCall(content, attemptReq.Tools, attemptReq.Functions) {
			responseID := utils.GenerateChatCompletionID()
			if toolCalls, textContent, ok := buildOpenAIToolCallsFromXML(content, attemptReq.Tools, attemptReq.Functions); ok {
				response := models.NewChatCompletionToolCallResponse(responseID, request.Model, toolCalls, textContent, usage)
				c.JSON(http.StatusOK, response)
				return true
			}
			response := models.NewChatCompletionResponse(responseID, request.Model, content, usage)
			c.JSON(http.StatusOK, response)
			return true
		}

		if attempt == maxToolBridgeRetries {
			break
		}

		// Roo-style: 把上一次 assistant 的非工具回复加入上下文，并注入“必须调用工具”的纠正消息
		if strings.TrimSpace(content) != "" {
			messages = append(messages, models.Message{Role: "assistant", Content: content})
		}
		messages = append(messages, models.Message{
			Role:    "user",
			Content: models.BuildNoToolsUsedRetryPrompt(attemptReq.Tools, attemptReq.Functions),
		})
	}

	responseID := utils.GenerateChatCompletionID()
	response := models.NewChatCompletionResponse(responseID, request.Model, lastContent, lastUsage)
	c.JSON(http.StatusOK, response)
	return true
}

func (h *Handler) handleStreamToolBridgeWithRetry(c *gin.Context, request *models.ChatCompletionRequest) bool {
	const maxToolBridgeRetries = 2

	messages := append([]models.Message(nil), request.Messages...)
	var lastUsage models.Usage
	var lastContent string

	for attempt := 0; attempt <= maxToolBridgeRetries; attempt++ {
		attemptReq := *request
		attemptReq.Messages = append([]models.Message(nil), messages...)
		attemptReq.Stream = true

		chatGenerator, err := h.cursorService.ChatCompletion(c.Request.Context(), &attemptReq)
		if err != nil {
			logrus.WithError(err).Error("Failed to create stream chat completion during tool bridge retry")
			middleware.HandleError(c, err)
			return true
		}

		// 缓存本轮流式输出，避免首轮失败内容提前发给客户端。
		content, usage, err := collectNonStreamResult(c, chatGenerator)
		if err != nil {
			middleware.HandleError(c, err)
			return true
		}

		lastContent = content
		lastUsage = usage

		if models.ContainsXMLToolCall(content, attemptReq.Tools, attemptReq.Functions) {
			if toolCalls, textContent, ok := buildOpenAIToolCallsFromXML(content, attemptReq.Tools, attemptReq.Functions); ok {
				streamBufferedToolCallResponse(c, request.Model, textContent, toolCalls, usage)
				return true
			}
			streamBufferedTextResponse(c, request.Model, content, usage)
			return true
		}

		if attempt == maxToolBridgeRetries {
			break
		}

		if strings.TrimSpace(content) != "" {
			messages = append(messages, models.Message{Role: "assistant", Content: content})
		}
		messages = append(messages, models.Message{
			Role:    "user",
			Content: models.BuildNoToolsUsedRetryPrompt(attemptReq.Tools, attemptReq.Functions),
		})
	}

	streamBufferedTextResponse(c, request.Model, lastContent, lastUsage)
	return true
}

func buildOpenAIToolCallsFromXML(content string, tools []models.ToolSpec, functions []models.FunctionSpec) ([]models.ToolCall, string, bool) {
	parsedFunctions, ok := models.ExtractXMLToolCalls(content, tools, functions)
	if !ok || len(parsedFunctions) == 0 {
		return nil, "", false
	}

	toolCalls := make([]models.ToolCall, 0, len(parsedFunctions))
	for _, fn := range parsedFunctions {
		toolCalls = append(toolCalls, models.ToolCall{
			ID:       "call_" + utils.GenerateRandomString(24),
			Type:     "function",
			Function: fn,
		})
	}

	textContent := models.ExtractNonToolTextFromXMLContent(content, tools, functions)
	return toolCalls, textContent, true
}

func streamBufferedTextResponse(c *gin.Context, model, content string, usage models.Usage) {
	buffered := make(chan interface{}, 2)
	if strings.TrimSpace(content) != "" {
		buffered <- content
	}
	buffered <- usage
	close(buffered)
	utils.StreamChatCompletion(c, buffered, model)
}

func streamBufferedToolCallResponse(c *gin.Context, model, content string, toolCalls []models.ToolCall, usage models.Usage) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	responseID := utils.GenerateChatCompletionID()

	writeChunk := func(delta models.StreamDelta, finishReason *string, usagePayload *models.Usage) bool {
		chunk := models.ChatCompletionStreamResponse{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []models.StreamChoice{
				{
					Index:        0,
					Delta:        delta,
					FinishReason: finishReason,
				},
			},
			Usage: usagePayload,
		}
		jsonData, err := json.Marshal(chunk)
		if err != nil {
			logrus.WithError(err).Error("Failed to marshal stream tool_call chunk")
			return false
		}
		if err := utils.WriteSSEEvent(c.Writer, "", string(jsonData)); err != nil {
			logrus.WithError(err).Error("Failed to write stream tool_call chunk")
			return false
		}
		return true
	}

	if !writeChunk(models.StreamDelta{Role: "assistant"}, nil, nil) {
		return
	}

	if strings.TrimSpace(content) != "" {
		if !writeChunk(models.StreamDelta{Content: content}, nil, nil) {
			return
		}
	}

	for idx, tc := range toolCalls {
		delta := models.StreamDelta{
			ToolCalls: []models.StreamToolCall{
				{
					Index: idx,
					ID:    tc.ID,
					Type:  tc.Type,
					Function: &models.StreamFunctionDelta{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				},
			},
		}
		if !writeChunk(delta, nil, nil) {
			return
		}
	}

	finishReason := "tool_calls"
	usageCopy := usage
	if !writeChunk(models.StreamDelta{}, &finishReason, &usageCopy) {
		return
	}
	if err := utils.WriteSSEEvent(c.Writer, "", "[DONE]"); err != nil {
		logrus.WithError(err).Error("Failed to write stream DONE event")
	}
}

func collectNonStreamResult(c *gin.Context, chatGenerator <-chan interface{}) (string, models.Usage, error) {
	var fullContent strings.Builder
	var usage models.Usage

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return "", usage, middleware.NewCursorWebError(http.StatusRequestTimeout, "request timeout")
		case data, ok := <-chatGenerator:
			if !ok {
				return fullContent.String(), usage, nil
			}
			switch v := data.(type) {
			case string:
				fullContent.WriteString(v)
			case models.Usage:
				usage = v
			case error:
				return "", usage, v
			}
		}
	}
}

// ServeDocs 服务API文档页面
func (h *Handler) ServeDocs(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", h.docsContent)
}

// Health 健康检查
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   "go-1.0.0",
	})
}
