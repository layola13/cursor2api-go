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

package services

import (
	"bytes"
	"context"
	"cursor2api-go/config"
	"cursor2api-go/middleware"
	"cursor2api-go/models"
	"cursor2api-go/utils"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

const cursorAPIURL = "https://cursor.com/api/chat"
const maxImageDataURLChars = 42000
const base64MirrorEnvKey = "CURSOR2API_ENABLE_IMAGE_BASE64_MIRROR"

var dataImageURLPattern = regexp.MustCompile(`(?is)data:image/[a-z0-9.+-]+;base64,[a-z0-9+/=\s]+`)
var base64BlockPattern = regexp.MustCompile(`(?is)BASE64_BEGIN\s*([a-z0-9+/=\s]+?)\s*BASE64_END`)
var longBase64Pattern = regexp.MustCompile(`(?is)[a-z0-9+/=\s]{512,}`)
var markdownImageURLPattern = regexp.MustCompile(`!\[[^\]]*]\((https?://[^\s)]+)\)`)
var httpURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

// CursorService handles interactions with Cursor API.
type CursorService struct {
	config          *config.Config
	client          *req.Client
	mainJS          string
	envJS           string
	scriptCache     string
	scriptCacheTime time.Time
	scriptMutex     sync.RWMutex
	headerGenerator *utils.HeaderGenerator
}

// NewCursorService creates a new service instance.
func NewCursorService(cfg *config.Config) *CursorService {
	mainJS, err := os.ReadFile(filepath.Join("jscode", "main.js"))
	if err != nil {
		logrus.Fatalf("failed to read jscode/main.js: %v", err)
	}

	envJS, err := os.ReadFile(filepath.Join("jscode", "env.js"))
	if err != nil {
		logrus.Fatalf("failed to read jscode/env.js: %v", err)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		logrus.Warnf("failed to create cookie jar: %v", err)
	}

	client := req.C()
	client.SetTimeout(time.Duration(cfg.Timeout) * time.Second)
	client.ImpersonateChrome()
	if jar != nil {
		client.SetCookieJar(jar)
	}

	return &CursorService{
		config:          cfg,
		client:          client,
		mainJS:          string(mainJS),
		envJS:           string(envJS),
		headerGenerator: utils.NewHeaderGenerator(),
	}
}

// ChatCompletion creates a chat completion stream for the given request.
func (s *CursorService) ChatCompletion(ctx context.Context, request *models.ChatCompletionRequest) (<-chan interface{}, error) {
	messages := request.Messages
	systemPromptInject := s.config.SystemPromptInject

	if models.NeedToolCallBridge(request) {
		messages = models.NormalizeMessagesForToolBridge(messages)
		if bridgePrompt := models.BuildToolCallBridgePrompt(request.Tools, request.Functions, request.ToolChoice, request.FunctionCall); bridgePrompt != "" {
			systemPromptInject = mergeSystemPrompts(systemPromptInject, bridgePrompt)
		}
	}
	if models.HasImageContent(messages) {
		systemPromptInject = mergeSystemPrompts(systemPromptInject, models.BuildImageInputBridgePrompt())
	}

	truncatedMessages := s.truncateMessages(messages)
	cursorMessages := models.ToCursorMessages(truncatedMessages, systemPromptInject)
	hasEmbeddedBase64 := s.expandEmbeddedImagePayloads(cursorMessages)
	if hasCursorImageParts(cursorMessages) {
		cursorMessages = ensureCursorSystemPrompt(cursorMessages, models.BuildImageInputBridgePrompt())
	}
	if hasEmbeddedBase64 {
		cursorMessages = ensureCursorSystemPrompt(cursorMessages, models.BuildBase64InputBridgePrompt())
	}
	s.normalizeCursorImageParts(ctx, cursorMessages)
	if s.appendBase64MirrorParts(cursorMessages) {
		cursorMessages = ensureCursorSystemPrompt(cursorMessages, models.BuildBase64InputBridgePrompt())
	}
	logCursorMessageSummary(cursorMessages)

	// 获取Cursor API使用的实际模型名称
	cursorModel := models.GetCursorModel(request.Model)

	payload := models.CursorRequest{
		Context:  []interface{}{},
		Model:    cursorModel,
		ID:       utils.GenerateRandomString(16),
		Messages: cursorMessages,
		Trigger:  "submit-message",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cursor payload: %w", err)
	}

	// 尝试最多2次
	maxRetries := 2
	for attempt := 1; attempt <= maxRetries; attempt++ {
		xIsHuman, err := s.fetchXIsHuman(ctx)
		if err != nil {
			if attempt < maxRetries {
				logrus.WithError(err).Warnf("Failed to fetch x-is-human token (attempt %d/%d), retrying...", attempt, maxRetries)
				time.Sleep(time.Second * time.Duration(attempt)) // 指数退避
				continue
			}
			return nil, err
		}

		// 添加详细的调试日志
		headers := s.chatHeaders(xIsHuman)
		logrus.WithFields(logrus.Fields{
			"url":            cursorAPIURL,
			"x-is-human":     xIsHuman[:50] + "...", // 只显示前50个字符
			"payload_length": len(jsonPayload),
			"model":          request.Model,
			"attempt":        attempt,
		}).Debug("Sending request to Cursor API")

		resp, err := s.client.R().
			SetContext(ctx).
			SetHeaders(headers).
			SetBody(jsonPayload).
			DisableAutoReadResponse().
			Post(cursorAPIURL)
		if err != nil {
			if attempt < maxRetries {
				logrus.WithError(err).Warnf("Cursor request failed (attempt %d/%d), retrying...", attempt, maxRetries)
				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}
			return nil, fmt.Errorf("cursor request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Response.Body)
			resp.Response.Body.Close()
			message := strings.TrimSpace(string(body))

			// 记录详细的错误信息
			logrus.WithFields(logrus.Fields{
				"status_code": resp.StatusCode,
				"response":    message,
				"headers":     resp.Header,
				"attempt":     attempt,
			}).Error("Cursor API returned non-OK status")

			// 如果是 403 错误且还有重试机会,清除缓存并重试
			if resp.StatusCode == http.StatusForbidden && attempt < maxRetries {
				logrus.Warn("Received 403 Access Denied, refreshing browser fingerprint and clearing token cache...")

				// 刷新浏览器指纹
				s.headerGenerator.Refresh()
				logrus.WithFields(logrus.Fields{
					"platform":       s.headerGenerator.GetProfile().Platform,
					"chrome_version": s.headerGenerator.GetProfile().ChromeVersion,
				}).Debug("Refreshed browser fingerprint")

				// 清除 token 缓存
				s.scriptMutex.Lock()
				s.scriptCache = ""
				s.scriptCacheTime = time.Time{}
				s.scriptMutex.Unlock()

				time.Sleep(time.Second * time.Duration(attempt))
				continue
			}

			if strings.Contains(message, "Attention Required! | Cloudflare") {
				message = "Cloudflare 403"
			}
			return nil, middleware.NewCursorWebError(resp.StatusCode, message)
		}

		// 成功,返回结果
		output := make(chan interface{}, 32)
		go s.consumeSSE(ctx, resp.Response, output)
		return output, nil
	}

	return nil, fmt.Errorf("failed after %d attempts", maxRetries)
}

func mergeSystemPrompts(base, appendText string) string {
	base = strings.TrimSpace(base)
	appendText = strings.TrimSpace(appendText)

	if base == "" {
		return appendText
	}
	if appendText == "" {
		return base
	}

	return base + "\n\n" + appendText
}

func ensureCursorSystemPrompt(messages []models.CursorMessage, prompt string) []models.CursorMessage {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return messages
	}

	if len(messages) > 0 && strings.EqualFold(strings.TrimSpace(messages[0].Role), "system") {
		if len(messages[0].Parts) == 0 {
			messages[0].Parts = []models.CursorPart{{Type: "text", Text: prompt}}
			return messages
		}
		for i := range messages[0].Parts {
			if strings.EqualFold(strings.TrimSpace(messages[0].Parts[i].Type), "text") {
				current := strings.TrimSpace(messages[0].Parts[i].Text)
				if current == "" {
					messages[0].Parts[i].Text = prompt
				} else {
					messages[0].Parts[i].Text = current + "\n\n" + prompt
				}
				return messages
			}
		}
		messages[0].Parts = append([]models.CursorPart{{Type: "text", Text: prompt}}, messages[0].Parts...)
		return messages
	}

	systemMsg := models.CursorMessage{
		Role: "system",
		Parts: []models.CursorPart{
			{Type: "text", Text: prompt},
		},
	}
	return append([]models.CursorMessage{systemMsg}, messages...)
}

func hasCursorImageParts(messages []models.CursorMessage) bool {
	for _, msg := range messages {
		for _, part := range msg.Parts {
			partType := strings.ToLower(strings.TrimSpace(part.Type))
			if partType == "image_url" && part.ImageURL != nil && strings.TrimSpace(part.ImageURL.URL) != "" {
				return true
			}
			if partType == "image" && part.Source != nil && strings.TrimSpace(part.Source.Data) != "" {
				return true
			}
		}
	}
	return false
}

func (s *CursorService) expandEmbeddedImagePayloads(messages []models.CursorMessage) bool {
	expanded := false

	for i := range messages {
		newParts := make([]models.CursorPart, 0, len(messages[i].Parts))
		for _, part := range messages[i].Parts {
			if !strings.EqualFold(strings.TrimSpace(part.Type), "text") || strings.TrimSpace(part.Text) == "" {
				newParts = append(newParts, part)
				continue
			}

			_, urls := extractImageDataURLsFromText(part.Text)
			if len(urls) == 0 {
				newParts = append(newParts, part)
				continue
			}

			expanded = true
			// Keep original text payload for model-side decoding robustness.
			newParts = append(newParts, part)
			for _, u := range urls {
				newParts = append(newParts, models.CursorPart{
					Type:     "image_url",
					ImageURL: &models.CursorImageURL{URL: u},
				})
			}
		}
		messages[i].Parts = newParts
	}

	return expanded
}

func extractImageDataURLsFromText(text string) (string, []string) {
	working := text
	found := make([]string, 0, 2)

	dataURLs := dataImageURLPattern.FindAllString(working, -1)
	for _, u := range dataURLs {
		normalized := normalizeDataURL(u)
		if media, data, ok := parseDataImageURL(normalized); ok && media != "" && data != "" {
			found = append(found, normalized)
		}
	}
	working = dataImageURLPattern.ReplaceAllString(working, "")

	blockMatches := base64BlockPattern.FindAllStringSubmatch(working, -1)
	for _, m := range blockMatches {
		if len(m) < 2 {
			continue
		}
		if dataURL, ok := base64PayloadToDataURL(m[1]); ok {
			found = append(found, dataURL)
		}
	}
	working = base64BlockPattern.ReplaceAllString(working, "")

	if len(found) == 0 {
		longCandidates := longBase64Pattern.FindAllString(working, -1)
		for _, c := range longCandidates {
			compactLen := len(strings.Map(func(r rune) rune {
				if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
					return -1
				}
				return r
			}, c))
			if compactLen < 2048 {
				continue
			}
			if dataURL, ok := base64PayloadToDataURL(c); ok {
				found = append(found, dataURL)
				working = strings.Replace(working, c, "", 1)
				break
			}
		}
	}

	linkURLs := extractImageLinksFromText(working)
	if len(linkURLs) > 0 {
		found = append(found, linkURLs...)
	}

	working = strings.TrimSpace(working)
	return working, uniqueStrings(found)
}

func extractImageLinksFromText(text string) []string {
	found := make([]string, 0, 2)

	for _, match := range markdownImageURLPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		raw := trimLikelyTrailingURLPunctuation(match[1])
		if isLikelyImageLink(raw, true) {
			found = append(found, raw)
		}
	}

	for _, rawURL := range httpURLPattern.FindAllString(text, -1) {
		raw := trimLikelyTrailingURLPunctuation(rawURL)
		if isLikelyImageLink(raw, false) {
			found = append(found, raw)
		}
	}

	return uniqueStrings(found)
}

func trimLikelyTrailingURLPunctuation(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), ".,;:!?)]}>'\"")
}

func isLikelyImageLink(raw string, fromMarkdown bool) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}

	if fromMarkdown {
		return true
	}

	switch strings.ToLower(filepath.Ext(parsed.Path)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".svg", ".tif", ".tiff", ".ico", ".avif", ".heic", ".heif":
		return true
	}

	query := parsed.Query()
	for _, key := range []string{"format", "fm", "ext", "type"} {
		value := strings.ToLower(strings.TrimSpace(query.Get(key)))
		switch value {
		case "png", "jpg", "jpeg", "webp", "gif", "bmp", "svg", "tif", "tiff", "ico", "avif", "heic", "heif", "image":
			return true
		}
	}

	return false
}

func normalizeDataURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if idx := strings.Index(trimmed, ","); idx > 0 && idx < len(trimmed)-1 {
		header := trimmed[:idx+1]
		body := strings.TrimSpace(trimmed[idx+1:])
		body = strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
				return -1
			}
			return r
		}, body)
		return header + body
	}
	return trimmed
}

func base64PayloadToDataURL(payload string) (string, bool) {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			return -1
		}
		return r
	}, payload)
	if len(clean) < 2048 {
		return "", false
	}
	data, err := base64.StdEncoding.DecodeString(clean)
	if err != nil || len(data) == 0 {
		return "", false
	}
	mimeType := detectImageMimeType(data, "", "")
	if !strings.HasPrefix(mimeType, "image/") {
		return "", false
	}
	return "data:" + mimeType + ";base64," + clean, true
}

func uniqueStrings(items []string) []string {
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

func (s *CursorService) normalizeCursorImageParts(ctx context.Context, messages []models.CursorMessage) {
	for i := range messages {
		for j := range messages[i].Parts {
			part := &messages[i].Parts[j]

			if strings.EqualFold(strings.TrimSpace(part.Type), "image") && part.ImageURL == nil && part.Source != nil {
				if strings.TrimSpace(part.Source.MediaType) != "" && strings.TrimSpace(part.Source.Data) != "" {
					part.ImageURL = &models.CursorImageURL{
						URL: "data:" + strings.TrimSpace(part.Source.MediaType) + ";base64," + strings.TrimSpace(part.Source.Data),
					}
					part.Image = strings.TrimSpace(part.Source.Data)
					part.MediaType = strings.TrimSpace(part.Source.MediaType)
				}
			}

			if part.ImageURL == nil {
				continue
			}

			rawURL := strings.TrimSpace(part.ImageURL.URL)
			if rawURL == "" {
				continue
			}

			dataURL := rawURL
			if !strings.HasPrefix(strings.ToLower(rawURL), "data:image/") {
				normalized, err := s.resolveImageURLToDataURL(ctx, rawURL)
				if err != nil {
					logrus.WithError(err).Warnf("Failed to normalize image url to data url: %s", rawURL)
					continue
				}
				dataURL = normalized
			}
			dataURL = normalizeDataURL(dataURL)
			part.ImageURL.URL = dataURL

			if mediaType, data, ok := parseDataImageURL(dataURL); ok {
				// Cursor backend accepts Anthropic-style image/source more reliably.
				part.Type = "image"
				part.Source = &models.CursorImageSource{
					Type:         "base64",
					MediaType:    mediaType,
					MediaTypeAlt: mediaType,
					Data:         data,
				}
				part.Image = data
				part.MediaType = mediaType
			}
		}
	}
}

func (s *CursorService) appendBase64MirrorParts(messages []models.CursorMessage) bool {
	if !isBase64MirrorEnabled() {
		return false
	}

	added := false
	for i := range messages {
		newParts := make([]models.CursorPart, 0, len(messages[i].Parts)+2)
		existingText := ""
		for _, part := range messages[i].Parts {
			if strings.EqualFold(strings.TrimSpace(part.Type), "text") {
				existingText += "\n" + part.Text
			}
		}
		for _, part := range messages[i].Parts {
			newParts = append(newParts, part)

			if part.Source == nil || strings.TrimSpace(part.Source.Data) == "" {
				continue
			}
			if strings.Contains(existingText, "BASE64_BEGIN") && strings.Contains(existingText, "BASE64_END") {
				continue
			}

			mediaType := strings.TrimSpace(part.Source.MediaType)
			if mediaType == "" {
				mediaType = "image/*"
			}

			added = true
			newParts = append(newParts, models.CursorPart{
				Type: "text",
				Text: "Attached image payload mirror (internal):\nBASE64_BEGIN\ndata:" + mediaType + ";base64," + part.Source.Data + "\nBASE64_END",
			})
		}
		messages[i].Parts = newParts
	}
	return added
}

func isBase64MirrorEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(base64MirrorEnvKey)))
	if value == "" {
		return true
	}
	if value == "0" || value == "false" || value == "no" || value == "off" {
		return false
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func parseDataImageURL(dataURL string) (mediaType string, data string, ok bool) {
	raw := strings.TrimSpace(dataURL)
	if !strings.HasPrefix(strings.ToLower(raw), "data:image/") {
		return "", "", false
	}
	comma := strings.Index(raw, ",")
	if comma <= 0 || comma >= len(raw)-1 {
		return "", "", false
	}

	header := raw[:comma]
	data = raw[comma+1:]
	if !strings.Contains(strings.ToLower(header), ";base64") {
		return "", "", false
	}
	mediaType = strings.TrimPrefix(strings.SplitN(header, ";", 2)[0], "data:")
	if mediaType == "" || data == "" {
		return "", "", false
	}
	return mediaType, data, true
}

func shrinkDataURLIfNeeded(dataURL string) string {
	if len(dataURL) <= maxImageDataURLChars {
		return dataURL
	}
	mediaType, data, ok := parseDataImageURL(dataURL)
	if !ok {
		return dataURL
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(raw) == 0 {
		return dataURL
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return dataURL
	}

	best := dataURL
	bestLen := len(dataURL)
	dimensions := []int{1400, 1200, 1024, 900, 768, 640, 512, 384, 320}
	qualities := []int{80, 70, 60, 50, 40, 35}
	for _, maxDim := range dimensions {
		resized := resizeImageToFit(img, maxDim, maxDim)
		for _, quality := range qualities {
			compressed, err := encodeJPEGWithQuality(resized, quality)
			if err != nil {
				continue
			}
			candidate := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(compressed)
			if len(candidate) < bestLen {
				best = candidate
				bestLen = len(candidate)
			}
			if len(candidate) <= maxImageDataURLChars {
				return candidate
			}
		}
	}

	// keep original media type if we did not improve enough with re-encode
	if bestLen >= len(dataURL) && strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return dataURL
	}
	return best
}

func encodeJPEGWithQuality(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func resizeImageToFit(src image.Image, maxWidth, maxHeight int) image.Image {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w <= 0 || h <= 0 || maxWidth <= 0 || maxHeight <= 0 {
		return src
	}
	if w <= maxWidth && h <= maxHeight {
		return src
	}

	scaleW := float64(maxWidth) / float64(w)
	scaleH := float64(maxHeight) / float64(h)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := b.Min.Y + int(float64(y)*float64(h)/float64(newH))
		if srcY >= b.Max.Y {
			srcY = b.Max.Y - 1
		}
		for x := 0; x < newW; x++ {
			srcX := b.Min.X + int(float64(x)*float64(w)/float64(newW))
			if srcX >= b.Max.X {
				srcX = b.Max.X - 1
			}
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func (s *CursorService) resolveImageURLToDataURL(ctx context.Context, rawURL string) (string, error) {
	switch {
	case strings.HasPrefix(strings.ToLower(rawURL), "http://"), strings.HasPrefix(strings.ToLower(rawURL), "https://"):
		return s.fetchRemoteImageAsDataURL(ctx, rawURL)
	case strings.HasPrefix(strings.ToLower(rawURL), "file://"):
		localPath := strings.TrimPrefix(rawURL, "file://")
		return readLocalImageAsDataURL(localPath)
	case strings.HasPrefix(rawURL, "/"), strings.HasPrefix(rawURL, "./"), strings.HasPrefix(rawURL, "../"):
		return readLocalImageAsDataURL(rawURL)
	default:
		return "", fmt.Errorf("unsupported image url format")
	}
}

func (s *CursorService) fetchRemoteImageAsDataURL(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", s.config.FP.UserAgent)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")

	client := &http.Client{Timeout: time.Duration(s.config.Timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty image response body")
	}

	mimeType := detectImageMimeType(data, resp.Header.Get("Content-Type"), rawURL)
	if !strings.HasPrefix(mimeType, "image/") {
		return "", fmt.Errorf("invalid image mime type: %s", mimeType)
	}

	dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	return dataURL, nil
}

func readLocalImageAsDataURL(localPath string) (string, error) {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty image file")
	}

	mimeType := detectImageMimeType(data, "", localPath)
	if !strings.HasPrefix(mimeType, "image/") {
		return "", fmt.Errorf("invalid image mime type: %s", mimeType)
	}

	dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	return dataURL, nil
}

func detectImageMimeType(data []byte, headerContentType, source string) string {
	if headerContentType != "" {
		if mediaType, _, err := mime.ParseMediaType(headerContentType); err == nil && mediaType != "" {
			return mediaType
		}
	}

	detected := http.DetectContentType(data)
	if strings.HasPrefix(detected, "image/") {
		return detected
	}

	ext := strings.ToLower(filepath.Ext(source))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	}

	// fallback to octet-stream to fail fast on caller side
	return "application/octet-stream"
}

func (s *CursorService) consumeSSE(ctx context.Context, resp *http.Response, output chan interface{}) {
	defer close(output)

	if err := utils.ReadSSEStream(ctx, resp, output); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		errResp := middleware.NewCursorWebError(http.StatusBadGateway, err.Error())
		select {
		case output <- errResp:
		default:
			logrus.WithError(err).Warn("failed to push SSE error to channel")
		}
	}
}

func (s *CursorService) fetchXIsHuman(ctx context.Context) (string, error) {
	// 检查缓存
	s.scriptMutex.RLock()
	cached := s.scriptCache
	lastFetch := s.scriptCacheTime
	s.scriptMutex.RUnlock()

	var scriptBody string
	// 缓存有效期缩短到1分钟,避免 token 过期
	if cached != "" && time.Since(lastFetch) < 1*time.Minute {
		scriptBody = cached
	} else {
		resp, err := s.client.R().
			SetContext(ctx).
			SetHeaders(s.scriptHeaders()).
			Get(s.config.ScriptURL)

		if err != nil {
			// 如果请求失败且有缓存，使用缓存
			if cached != "" {
				logrus.Warnf("Failed to fetch script, using cached version: %v", err)
				scriptBody = cached
			} else {
				// 清除缓存并生成一个简单的token
				s.scriptMutex.Lock()
				s.scriptCache = ""
				s.scriptCacheTime = time.Time{}
				s.scriptMutex.Unlock()
				// 生成一个简单的x-is-human token作为fallback
				token := utils.GenerateRandomString(64)
				logrus.Warnf("Failed to fetch script, generated fallback token")
				return token, nil
			}
		} else if resp.StatusCode != http.StatusOK {
			// 如果状态码异常且有缓存，使用缓存
			if cached != "" {
				logrus.Warnf("Script fetch returned status %d, using cached version", resp.StatusCode)
				scriptBody = cached
			} else {
				// 清除缓存并生成一个简单的token
				s.scriptMutex.Lock()
				s.scriptCache = ""
				s.scriptCacheTime = time.Time{}
				s.scriptMutex.Unlock()
				// 生成一个简单的x-is-human token作为fallback
				token := utils.GenerateRandomString(64)
				logrus.Warnf("Script fetch returned status %d, generated fallback token", resp.StatusCode)
				return token, nil
			}
		} else {
			scriptBody = string(resp.Bytes())
			// 更新缓存
			s.scriptMutex.Lock()
			s.scriptCache = scriptBody
			s.scriptCacheTime = time.Now()
			s.scriptMutex.Unlock()
		}
	}

	compiled := s.prepareJS(scriptBody)
	value, err := utils.RunJS(compiled)
	if err != nil {
		// JS 执行失败时清除缓存并生成fallback token
		s.scriptMutex.Lock()
		s.scriptCache = ""
		s.scriptCacheTime = time.Time{}
		s.scriptMutex.Unlock()
		token := utils.GenerateRandomString(64)
		logrus.Warnf("Failed to execute JS, generated fallback token: %v", err)
		return token, nil
	}

	logrus.WithField("length", len(value)).Debug("Fetched x-is-human token")

	return value, nil
}

func (s *CursorService) prepareJS(cursorJS string) string {
	replacer := strings.NewReplacer(
		"$$currentScriptSrc$$", s.config.ScriptURL,
		"$$UNMASKED_VENDOR_WEBGL$$", s.config.FP.UNMASKED_VENDOR_WEBGL,
		"$$UNMASKED_RENDERER_WEBGL$$", s.config.FP.UNMASKED_RENDERER_WEBGL,
		"$$userAgent$$", s.config.FP.UserAgent,
	)

	mainScript := replacer.Replace(s.mainJS)
	mainScript = strings.Replace(mainScript, "$$env_jscode$$", s.envJS, 1)
	mainScript = strings.Replace(mainScript, "$$cursor_jscode$$", cursorJS, 1)
	return mainScript
}

func (s *CursorService) truncateMessages(messages []models.Message) []models.Message {
	if len(messages) == 0 || s.config.MaxInputLength <= 0 {
		return messages
	}

	maxLength := s.config.MaxInputLength
	total := 0
	for _, msg := range messages {
		total += estimateMessageInputLength(msg)
	}

	if total <= maxLength {
		return messages
	}

	var result []models.Message
	startIdx := 0

	if strings.EqualFold(messages[0].Role, "system") {
		result = append(result, messages[0])
		maxLength -= len(messages[0].GetStringContent())
		if maxLength < 0 {
			maxLength = 0
		}
		startIdx = 1
	}

	current := 0
	collected := make([]models.Message, 0, len(messages)-startIdx)
	for i := len(messages) - 1; i >= startIdx; i-- {
		msg := messages[i]
		msgLen := estimateMessageInputLength(msg)
		if msgLen <= 0 {
			continue
		}
		if current+msgLen > maxLength {
			// Never silently drop the newest message: keep it and let upstream validate limits.
			if i == len(messages)-1 && len(collected) == 0 {
				collected = append(collected, msg)
				current += msgLen
			}
			continue
		}
		collected = append(collected, msg)
		current += msgLen
	}

	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}

	return append(result, collected...)
}

func estimateMessageInputLength(msg models.Message) int {
	total := len(msg.GetStringContent())

	cursorMessages := models.ToCursorMessages([]models.Message{msg}, "")
	for _, cursorMsg := range cursorMessages {
		for _, part := range cursorMsg.Parts {
			if part.ImageURL != nil {
				total += len(strings.TrimSpace(part.ImageURL.URL))
			}
		}
	}

	return total
}

func logCursorMessageSummary(messages []models.CursorMessage) {
	if !logrus.IsLevelEnabled(logrus.DebugLevel) {
		return
	}

	type messageSummary struct {
		Role         string `json:"role"`
		Parts        int    `json:"parts"`
		TextChars    int    `json:"text_chars"`
		ImageParts   int    `json:"image_parts"`
		ImageURLChars int   `json:"image_url_chars"`
	}

	summaries := make([]messageSummary, 0, len(messages))
	for _, msg := range messages {
		s := messageSummary{
			Role:  msg.Role,
			Parts: len(msg.Parts),
		}
		for _, part := range msg.Parts {
			if strings.EqualFold(strings.TrimSpace(part.Type), "text") {
				s.TextChars += len(part.Text)
			}
			if part.ImageURL != nil {
				s.ImageParts++
				s.ImageURLChars += len(strings.TrimSpace(part.ImageURL.URL))
			}
		}
		summaries = append(summaries, s)
	}

	logrus.WithField("cursor_message_summary", summaries).Debug("Prepared cursor messages")
}

func (s *CursorService) chatHeaders(xIsHuman string) map[string]string {
	return s.headerGenerator.GetChatHeaders(xIsHuman)
}

func (s *CursorService) scriptHeaders() map[string]string {
	return s.headerGenerator.GetScriptHeaders()
}
