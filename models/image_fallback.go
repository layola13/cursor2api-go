package models

import (
	"encoding/json"
	"strings"
)

// ExtractImageCandidatesFromRawRequest 尝试从原始请求体中提取图片候选 URL / dataURL。
// 用于兼容非标准客户端字段（如 CherryStudio 的扩展字段）。
func ExtractImageCandidatesFromRawRequest(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}

	var payload interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	found := make([]string, 0, 4)
	collectImageCandidates(payload, "", &found)
	return uniqueNonEmptyStrings(found)
}

func collectImageCandidates(node interface{}, parentKey string, out *[]string) {
	switch typed := node.(type) {
	case map[string]interface{}:
		if mediaType, ok := typed["media_type"].(string); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(mediaType)), "image/") {
			if data, ok := typed["data"].(string); ok {
				data = strings.TrimSpace(data)
				if data != "" {
					*out = append(*out, "data:"+strings.TrimSpace(mediaType)+";base64,"+data)
				}
			}
		}
		if mediaType, ok := typed["mediaType"].(string); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(mediaType)), "image/") {
			if data, ok := typed["image"].(string); ok {
				data = strings.TrimSpace(data)
				if data != "" && !strings.HasPrefix(strings.ToLower(data), "http://") && !strings.HasPrefix(strings.ToLower(data), "https://") && !strings.HasPrefix(strings.ToLower(data), "data:image/") {
					*out = append(*out, "data:"+strings.TrimSpace(mediaType)+";base64,"+data)
				}
			}
		}

		for key, value := range typed {
			lowerKey := strings.ToLower(strings.TrimSpace(key))
			if s, ok := value.(string); ok && isLikelyImageString(s, lowerKey) {
				*out = append(*out, strings.TrimSpace(s))
			}
			if lowerKey == "image_url" || lowerKey == "input_image" {
				if url := extractURLField(value); url != "" {
					*out = append(*out, url)
				}
			}
			collectImageCandidates(value, lowerKey, out)
		}

	case []interface{}:
		for _, item := range typed {
			collectImageCandidates(item, parentKey, out)
		}

	case string:
		if isLikelyImageString(typed, parentKey) {
			*out = append(*out, strings.TrimSpace(typed))
		}
	}
}

func isLikelyImageString(raw, parentKey string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "data:image/") {
		return true
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		switch parentKey {
		case "url", "href", "src", "image", "image_url", "input_image", "file_url", "content_url", "thumbnail", "images", "attachments", "files":
			return true
		}
		for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".svg", ".tif", ".tiff", ".ico", ".avif", ".heic", ".heif"} {
			if strings.Contains(lower, ext) {
				return true
			}
		}
	}
	return false
}

// InjectImageCandidatesIntoMessages 将提取到的图片候选注入到消息中（优先注入最后一条 user 消息）。
func InjectImageCandidatesIntoMessages(messages []Message, imageURLs []string) []Message {
	imageURLs = uniqueNonEmptyStrings(imageURLs)
	if len(imageURLs) == 0 {
		return messages
	}

	target := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(messages[i].Role), "user") {
			target = i
			break
		}
	}
	if target < 0 && len(messages) > 0 {
		target = len(messages) - 1
	}
	if target < 0 {
		messages = append(messages, Message{Role: "user", Content: ""})
		target = 0
	}

	msg := messages[target]
	switch content := msg.Content.(type) {
	case nil:
		parts := make([]interface{}, 0, len(imageURLs))
		for _, u := range imageURLs {
			parts = append(parts, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": u,
				},
			})
		}
		msg.Content = parts

	case string:
		parts := make([]interface{}, 0, len(imageURLs)+1)
		if strings.TrimSpace(content) != "" {
			parts = append(parts, map[string]interface{}{
				"type": "text",
				"text": content,
			})
		}
		for _, u := range imageURLs {
			parts = append(parts, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": u,
				},
			})
		}
		msg.Content = parts

	case []ContentPart:
		for _, u := range imageURLs {
			content = append(content, ContentPart{
				Type: "image_url",
				URL:  u,
			})
		}
		msg.Content = content

	case []interface{}:
		for _, u := range imageURLs {
			content = append(content, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url": u,
				},
			})
		}
		msg.Content = content
	}

	messages[target] = msg
	return messages
}
