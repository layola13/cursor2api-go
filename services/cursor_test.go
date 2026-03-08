package services

import (
	"context"
	"cursor2api-go/config"
	"cursor2api-go/models"
	"strings"
	"testing"
)

func TestExtractImageDataURLsFromTextWithImageLinks(t *testing.T) {
	input := "请识别这张图 ![img](https://example.com/a.png) 并参考 https://cdn.example.com/b.webp 和 https://example.com/page"
	_, urls := extractImageDataURLsFromText(input)

	if len(urls) < 2 {
		t.Fatalf("expected at least 2 urls, got %d: %#v", len(urls), urls)
	}

	assertContainsURL(t, urls, "https://example.com/a.png")
	assertContainsURL(t, urls, "https://cdn.example.com/b.webp")
	assertNotContainsURL(t, urls, "https://example.com/page")
}

func TestExtractImageDataURLsFromTextWithQueryImageLink(t *testing.T) {
	input := "请处理这个链接 https://img.example.com/render?id=1&format=png"
	_, urls := extractImageDataURLsFromText(input)
	assertContainsURL(t, urls, "https://img.example.com/render?id=1&format=png")
}

func TestExtractImageDataURLsFromTextTrimPunctuation(t *testing.T) {
	input := "看这个 https://example.com/a.jpg)."
	_, urls := extractImageDataURLsFromText(input)
	assertContainsURL(t, urls, "https://example.com/a.jpg")
}

func TestAppendBase64MirrorPartsCanBeDisabledByEnv(t *testing.T) {
	t.Setenv(base64MirrorEnvKey, "false")

	svc := &CursorService{}
	messages := []models.CursorMessage{
		{
			Role: "user",
			Parts: []models.CursorPart{
				{Type: "text", Text: "请分析这张图"},
				{
					Type: "image_url",
					ImageURL: &models.CursorImageURL{
						URL: "data:image/jpeg;base64,AAAABBBB",
					},
					Source: &models.CursorImageSource{
						Type:      "base64",
						MediaType: "image/jpeg",
						Data:      "AAAABBBB",
					},
				},
			},
		},
	}

	added := svc.appendBase64MirrorParts(messages)
	if added {
		t.Fatalf("appendBase64MirrorParts() = true, want false when env is explicitly disabled")
	}
	if len(messages[0].Parts) != 2 {
		t.Fatalf("parts len = %d, want 2", len(messages[0].Parts))
	}
}

func TestAppendBase64MirrorPartsEnabledWithEnv(t *testing.T) {
	t.Setenv(base64MirrorEnvKey, "true")

	svc := &CursorService{}
	messages := []models.CursorMessage{
		{
			Role: "user",
			Parts: []models.CursorPart{
				{Type: "text", Text: "请分析这张图"},
				{
					Type: "image",
					Source: &models.CursorImageSource{
						Type:      "base64",
						MediaType: "image/jpeg",
						Data:      "AAAABBBB",
					},
				},
			},
		},
	}

	added := svc.appendBase64MirrorParts(messages)
	if !added {
		t.Fatalf("appendBase64MirrorParts() = false, want true")
	}
	if len(messages[0].Parts) != 3 {
		t.Fatalf("parts len = %d, want 3", len(messages[0].Parts))
	}
	last := messages[0].Parts[2]
	if last.Type != "text" || !strings.Contains(last.Text, "BASE64_BEGIN") || !strings.Contains(last.Text, "BASE64_END") {
		t.Fatalf("unexpected mirror part: %#v", last)
	}
}

func TestAppendBase64MirrorPartsSkipWhenAlreadyPresent(t *testing.T) {
	t.Setenv(base64MirrorEnvKey, "1")

	svc := &CursorService{}
	messages := []models.CursorMessage{
		{
			Role: "user",
			Parts: []models.CursorPart{
				{Type: "text", Text: "BASE64_BEGIN\nAAAA\nBASE64_END"},
				{
					Type: "image",
					Source: &models.CursorImageSource{
						Type:      "base64",
						MediaType: "image/jpeg",
						Data:      "AAAABBBB",
					},
				},
			},
		},
	}

	added := svc.appendBase64MirrorParts(messages)
	if added {
		t.Fatalf("appendBase64MirrorParts() = true, want false")
	}
	if len(messages[0].Parts) != 2 {
		t.Fatalf("parts len = %d, want 2", len(messages[0].Parts))
	}
}

func TestNormalizeCursorImagePartsConvertsImageURLToSourceImage(t *testing.T) {
	svc := &CursorService{}
	messages := []models.CursorMessage{
		{
			Role: "user",
			Parts: []models.CursorPart{
				{
					Type: "image_url",
					ImageURL: &models.CursorImageURL{
						URL: "data:image/png;base64,AAAA",
					},
				},
			},
		},
	}

	svc.normalizeCursorImageParts(context.Background(), messages)

	part := messages[0].Parts[0]
	if part.Type != "image" {
		t.Fatalf("part.Type = %q, want image", part.Type)
	}
	if part.Source == nil {
		t.Fatalf("part.Source should not be nil")
	}
	if part.Source.MediaType != "image/png" || part.Source.Data != "AAAA" {
		t.Fatalf("unexpected source: %#v", part.Source)
	}
	if part.ImageURL == nil || part.ImageURL.URL != "data:image/png;base64,AAAA" {
		t.Fatalf("unexpected image url: %#v", part.ImageURL)
	}
}

func TestNormalizeCursorImagePartsConvertsSourceImageToImageURL(t *testing.T) {
	svc := &CursorService{}
	messages := []models.CursorMessage{
		{
			Role: "user",
			Parts: []models.CursorPart{
				{
					Type: "image",
					Source: &models.CursorImageSource{
						Type:      "base64",
						MediaType: "image/jpeg",
						Data:      "BBBB",
					},
				},
			},
		},
	}

	svc.normalizeCursorImageParts(context.Background(), messages)

	part := messages[0].Parts[0]
	if part.Type != "image" {
		t.Fatalf("part.Type = %q, want image", part.Type)
	}
	if part.ImageURL == nil || part.ImageURL.URL != "data:image/jpeg;base64,BBBB" {
		t.Fatalf("unexpected image url: %#v", part.ImageURL)
	}
	if part.Source == nil {
		t.Fatalf("part.Source should not be nil after normalization")
	}
	if part.Source.MediaType != "image/jpeg" || part.Source.Data != "BBBB" {
		t.Fatalf("unexpected source: %#v", part.Source)
	}
}

func TestTruncateMessagesKeepsNewestOversizedImageMessage(t *testing.T) {
	svc := &CursorService{
		config: &config.Config{
			MaxInputLength: 120,
		},
	}

	longText := strings.Repeat("x", 100)
	oversizedImage := "data:image/png;base64," + strings.Repeat("A", 400)
	messages := []models.Message{
		{Role: "user", Content: longText},
		{Role: "assistant", Content: "ok"},
		{Role: "user", Images: []string{oversizedImage}},
	}

	out := svc.truncateMessages(messages)
	if len(out) == 0 {
		t.Fatalf("truncateMessages() returned empty")
	}

	last := out[len(out)-1]
	if !models.HasImageContent([]models.Message{last}) {
		t.Fatalf("expected newest image message to be kept, got %#v", last)
	}
}

func assertContainsURL(t *testing.T, urls []string, want string) {
	t.Helper()
	for _, u := range urls {
		if u == want {
			return
		}
	}
	t.Fatalf("expected url %q not found in %#v", want, urls)
}

func assertNotContainsURL(t *testing.T, urls []string, bad string) {
	t.Helper()
	for _, u := range urls {
		if u == bad {
			t.Fatalf("unexpected url %q found in %#v", bad, urls)
		}
	}
}
