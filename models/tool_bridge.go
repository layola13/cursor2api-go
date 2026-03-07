package models

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var xmlTagSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
var xmlGenericBlockPattern = regexp.MustCompile(`(?s)<([a-zA-Z_][a-zA-Z0-9_.-]*)>(.*?)</([a-zA-Z_][a-zA-Z0-9_.-]*)>`)
var multiBlankLinePattern = regexp.MustCompile(`\n{3,}`)

// NeedToolCallBridge 判断是否需要将原生 tool/function 调用桥接为文本协议
func NeedToolCallBridge(request *ChatCompletionRequest) bool {
	if request == nil {
		return false
	}

	if len(request.Tools) > 0 || len(request.Functions) > 0 {
		return true
	}
	if request.ToolChoice != nil || request.FunctionCall != nil || request.ParallelToolCalls != nil {
		return true
	}

	for _, msg := range request.Messages {
		if len(msg.ToolCalls) > 0 || msg.FunctionCall != nil {
			return true
		}
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if role == "tool" || role == "function" {
			return true
		}
	}

	return false
}

// BuildToolCallBridgePrompt 生成 tool/function 桥接系统提示
func BuildToolCallBridgePrompt(tools []ToolSpec, functions []FunctionSpec, toolChoice interface{}, functionCall interface{}) string {
	specs := collectFunctionSpecs(tools, functions)

	if len(specs) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("The client requested OpenAI function/tool calling, but this backend does NOT support OpenAI function_call/tool_calls JSON outputs.\n")
	builder.WriteString("Use XML tool-calling protocol only.\n")
	builder.WriteString("This is a tool-enabled turn. Your next response should be an XML tool call when the request is actionable with listed tools.\n")
	if choiceInstruction := buildToolChoiceInstruction(toolChoice, functionCall); choiceInstruction != "" {
		builder.WriteString(choiceInstruction)
		builder.WriteByte('\n')
	}
	builder.WriteString("\n")
	builder.WriteString("Tool Use Formatting (Roo-style XML):\n")
	builder.WriteString("<actual_tool_name>\n")
	builder.WriteString("<parameter1_name>value1</parameter1_name>\n")
	builder.WriteString("<parameter2_name>value2</parameter2_name>\n")
	builder.WriteString("</actual_tool_name>\n")
	builder.WriteString("\n")
	builder.WriteString("Rules:\n")
	builder.WriteString("1) You ARE tool-enabled in this turn. Never claim tools are unavailable.\n")
	builder.WriteString("2) If the user asks for actions like reading/writing files, running commands, browsing, or external operations, you MUST call a tool first.\n")
	builder.WriteString("3) Use the actual tool name as the XML tag name.\n")
	builder.WriteString("4) Put each parameter in its own XML child tag. Do NOT send a JSON object directly inside the tool tag.\n")
	builder.WriteString("5) For object/array parameter values, serialize strict JSON and wrap with CDATA.\n")
	builder.WriteString("6) For text containing XML-sensitive chars (<, >, &) or multi-line content, use CDATA.\n")
	builder.WriteString("7) Return XML only when doing a tool call (no markdown, no prose, no code fences).\n")
	builder.WriteString("8) For multiple calls in one response, wrap calls in <tool_calls>...</tool_calls>.\n")
	builder.WriteString("9) DO NOT output OpenAI JSON blocks like {\"tool_calls\":...} or {\"function_call\":...}.\n")
	builder.WriteString("\n")
	builder.WriteString("中文说明：后端不支持原生 function_call/tool_calls JSON，必须改用 XML 工具调用。\n")
	builder.WriteString("你在本轮有可用工具，不要说“无法写文件/无法执行命令”。\n")
	builder.WriteString("遇到写文件、执行命令、检索等可操作任务，必须优先发起工具调用；本轮下一条回复应当是工具调用。\n")
	builder.WriteString("调用工具时只输出 XML，不要输出解释。\n")
	builder.WriteString("每个参数都必须是独立子标签；对象/数组参数请用 JSON + CDATA。\n")
	builder.WriteString("\n")
	builder.WriteString("Available tools:\n")

	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			continue
		}
		tagName := sanitizeXMLTagName(name)
		builder.WriteString(fmt.Sprintf("- <%s>\n", tagName))
		if desc := strings.TrimSpace(spec.Description); desc != "" {
			builder.WriteString(fmt.Sprintf("  description: %s\n", desc))
		}
		builder.WriteString("  xml_example:\n")
		builder.WriteString(indentLines(buildToolXMLTemplate(spec), "    "))
		builder.WriteByte('\n')
		if len(spec.Parameters) > 0 {
			builder.WriteString("  parameters: ")
			builder.WriteString(compactJSONOrRaw(spec.Parameters))
			builder.WriteByte('\n')
			required := extractRequiredParameterNames(spec.Parameters)
			if len(required) > 0 {
				builder.WriteString("  required: ")
				builder.WriteString(strings.Join(required, ", "))
				builder.WriteByte('\n')
			}
		}
	}

	return strings.TrimSpace(builder.String())
}

// NormalizeMessagesForToolBridge 将不兼容的 tool/function 消息转为文本消息，避免上下文丢失
func NormalizeMessagesForToolBridge(messages []Message) []Message {
	result := make([]Message, 0, len(messages))

	for _, msg := range messages {
		normalized := msg
		role := strings.ToLower(strings.TrimSpace(msg.Role))

		switch role {
		case "assistant":
			callText := buildAssistantToolCallText(msg)
			if callText != "" {
				content := strings.TrimSpace(msg.GetStringContent())
				if content == "" {
					normalized.Content = callText
				} else {
					normalized.Content = content + "\n" + callText
				}
			}
			normalized.ToolCalls = nil
			normalized.FunctionCall = nil
			result = append(result, normalized)

		case "tool", "function":
			normalized.Role = "user"
			normalized.ToolCallID = nil
			normalized.ToolCalls = nil
			normalized.FunctionCall = nil
			normalized.Content = wrapToolResultAsXML(msg)
			result = append(result, normalized)

		default:
			result = append(result, normalized)
		}
	}

	return result
}

func buildAssistantToolCallText(msg Message) string {
	var items []string

	for _, tc := range msg.ToolCalls {
		name := sanitizeXMLTagName(tc.Function.Name)
		items = append(items, buildXMLToolCall(name, tc.Function.Arguments))
	}

	if msg.FunctionCall != nil {
		name := sanitizeXMLTagName(msg.FunctionCall.Name)
		items = append(items, buildXMLToolCall(name, msg.FunctionCall.Arguments))
	}

	if len(items) == 0 {
		return ""
	}

	if len(items) == 1 {
		return items[0]
	}

	return "<tool_calls>\n" + strings.Join(items, "\n") + "\n</tool_calls>"
}

func wrapToolResultAsXML(msg Message) string {
	content := strings.TrimSpace(msg.GetStringContent())
	if content == "" {
		content = "(empty tool result)"
	}
	content = wrapCDATA(content)

	if msg.ToolCallID == nil || strings.TrimSpace(*msg.ToolCallID) == "" {
		return fmt.Sprintf("<tool_result>%s</tool_result>", content)
	}

	return fmt.Sprintf("<tool_result id=\"%s\">%s</tool_result>", escapeXMLAttr(strings.TrimSpace(*msg.ToolCallID)), content)
}

func sanitizeXMLTagName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "tool_call"
	}

	sanitized := xmlTagSanitizer.ReplaceAllString(trimmed, "_")
	if sanitized == "" {
		return "tool_call"
	}

	if sanitized[0] >= '0' && sanitized[0] <= '9' {
		return "tool_" + sanitized
	}

	return sanitized
}

func compactJSONOrRaw(raw json.RawMessage) string {
	var any interface{}
	if err := json.Unmarshal(raw, &any); err != nil {
		return strings.TrimSpace(string(raw))
	}

	data, err := json.Marshal(any)
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	return string(data)
}

func buildToolXMLTemplate(spec FunctionSpec) string {
	toolName := sanitizeXMLTagName(spec.Name)
	params := extractParameterNames(spec.Parameters)
	if len(params) == 0 {
		return fmt.Sprintf("<%s></%s>", toolName, toolName)
	}

	var lines []string
	lines = append(lines, "<"+toolName+">")
	for _, param := range params {
		lines = append(lines, fmt.Sprintf("<%s>...</%s>", sanitizeXMLTagName(param), sanitizeXMLTagName(param)))
	}
	lines = append(lines, "</"+toolName+">")
	return strings.Join(lines, "\n")
}

func buildXMLToolCall(toolName, rawArgs string) string {
	body := buildXMLParameters(rawArgs)
	if body == "" {
		return fmt.Sprintf("<%s></%s>", toolName, toolName)
	}
	return fmt.Sprintf("<%s>\n%s\n</%s>", toolName, body, toolName)
}

func buildXMLParameters(rawArgs string) string {
	trimmed := strings.TrimSpace(rawArgs)
	if trimmed == "" || trimmed == "{}" {
		return ""
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return "<arguments>" + wrapCDATA(trimmed) + "</arguments>"
	}
	if len(args) == 0 {
		return ""
	}

	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		tag := sanitizeXMLTagName(key)
		parts = append(parts, fmt.Sprintf("<%s>%s</%s>", tag, formatParameterValue(args[key]), tag))
	}
	return strings.Join(parts, "\n")
}

func formatParameterValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		if shouldUseCDATA(typed) {
			return wrapCDATA(typed)
		}
		return escapeXMLText(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return fmt.Sprintf("%v", typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return wrapCDATA(fmt.Sprintf("%v", typed))
		}
		return wrapCDATA(string(data))
	}
}

func shouldUseCDATA(value string) bool {
	if value == "" {
		return false
	}
	return strings.ContainsAny(value, "<>&\n\r\t") || strings.Contains(value, "]]>")
}

func wrapCDATA(value string) string {
	safe := strings.ReplaceAll(value, "]]>", "]]]]><![CDATA[>")
	return "<![CDATA[" + safe + "]]>"
}

func escapeXMLText(value string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return value
	}
	return buf.String()
}

func escapeXMLAttr(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		`"`, "&quot;",
		"<", "&lt;",
		">", "&gt;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func extractParameterNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return nil
	}

	names := make([]string, 0, len(properties))
	for key := range properties {
		names = append(names, key)
	}
	sort.Strings(names)
	return names
}

func extractRequiredParameterNames(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}

	requiredRaw, ok := schema["required"].([]interface{})
	if !ok || len(requiredRaw) == 0 {
		return nil
	}

	required := make([]string, 0, len(requiredRaw))
	for _, item := range requiredRaw {
		if name, ok := item.(string); ok && strings.TrimSpace(name) != "" {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	return required
}

func indentLines(text, indent string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

func BuildNoToolsUsedRetryPrompt(tools []ToolSpec, functions []FunctionSpec) string {
	specs := collectFunctionSpecs(tools, functions)
	if len(specs) == 0 {
		return "[ERROR] You did not use a tool in your previous response. Please retry with an XML tool call."
	}

	var builder strings.Builder
	builder.WriteString("[ERROR] You did not use a tool in your previous response! Please retry with a tool use.\n")
	builder.WriteString("You MUST return an XML tool call next. Do not reply with explanation text.\n\n")
	builder.WriteString("Tool format reminder:\n")
	builder.WriteString("<actual_tool_name>\n")
	builder.WriteString("<parameter1_name>value1</parameter1_name>\n")
	builder.WriteString("</actual_tool_name>\n\n")
	builder.WriteString("For multiple calls, use <tool_calls>...</tool_calls>.\n")
	builder.WriteString("Do NOT output JSON function_call/tool_calls blocks.\n")
	builder.WriteString("中文说明：上一条没有调用工具。下一条必须输出 XML 工具调用，不要解释文字。\n")
	builder.WriteString("Available tools this turn:\n")
	for _, spec := range specs {
		builder.WriteString("- <")
		builder.WriteString(sanitizeXMLTagName(spec.Name))
		builder.WriteString(">\n")
	}

	return strings.TrimSpace(builder.String())
}

func ContainsXMLToolCall(content string, tools []ToolSpec, functions []FunctionSpec) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "<tool_calls>") && strings.Contains(trimmed, "</tool_calls>") {
		return true
	}

	for _, spec := range collectFunctionSpecs(tools, functions) {
		tag := sanitizeXMLTagName(spec.Name)
		if tag == "" {
			continue
		}
		openTag := "<" + tag
		closeTag := "</" + tag + ">"
		if strings.Contains(trimmed, openTag) && strings.Contains(trimmed, closeTag) {
			return true
		}
	}
	return false
}

func ExtractXMLToolCalls(content string, tools []ToolSpec, functions []FunctionSpec) ([]Function, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, false
	}

	specs := collectFunctionSpecs(tools, functions)
	if len(specs) == 0 {
		return nil, false
	}

	toolNameByTag := make(map[string]string, len(specs))
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			continue
		}
		toolNameByTag[sanitizeXMLTagName(name)] = name
	}
	if len(toolNameByTag) == 0 {
		return nil, false
	}

	body := extractToolCallsBody(trimmed)
	if body == "" {
		body = trimmed
	}

	type matchedCall struct {
		Start int
		End   int
		Tag   string
		Body  string
	}
	var matched []matchedCall
	for tag := range toolNameByTag {
		pattern := regexp.MustCompile(`(?s)<` + regexp.QuoteMeta(tag) + `(?:\s+[^>]*)?>(.*?)</` + regexp.QuoteMeta(tag) + `>`)
		indexes := pattern.FindAllStringSubmatchIndex(body, -1)
		for _, idx := range indexes {
			if len(idx) < 4 {
				continue
			}
			matched = append(matched, matchedCall{
				Start: idx[0],
				End:   idx[1],
				Tag:   tag,
				Body:  body[idx[2]:idx[3]],
			})
		}
	}
	if len(matched) == 0 {
		return nil, false
	}

	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].Start < matched[j].Start
	})

	calls := make([]Function, 0, len(matched))
	lastEnd := -1
	for _, item := range matched {
		if item.Start < lastEnd {
			// 跳过重叠匹配，避免重复提取
			continue
		}
		lastEnd = item.End

		argsJSON := parseToolArgumentsXML(item.Body)
		originalName := toolNameByTag[item.Tag]
		if originalName == "" {
			originalName = item.Tag
		}
		calls = append(calls, Function{
			Name:      originalName,
			Arguments: argsJSON,
		})
	}

	return calls, len(calls) > 0
}

// ExtractNonToolTextFromXMLContent 提取非 XML 工具调用的普通文本说明。
func ExtractNonToolTextFromXMLContent(content string, tools []ToolSpec, functions []FunctionSpec) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	specs := collectFunctionSpecs(tools, functions)
	if len(specs) == 0 {
		return trimmed
	}

	cleaned := trimmed
	cleaned = strings.ReplaceAll(cleaned, "<tool_calls>", "")
	cleaned = strings.ReplaceAll(cleaned, "</tool_calls>", "")

	for _, spec := range specs {
		tag := sanitizeXMLTagName(spec.Name)
		if tag == "" {
			continue
		}
		pattern := regexp.MustCompile(`(?s)<` + regexp.QuoteMeta(tag) + `(?:\s+[^>]*)?>.*?</` + regexp.QuoteMeta(tag) + `>`)
		cleaned = pattern.ReplaceAllString(cleaned, "")
	}

	cleaned = strings.TrimSpace(cleaned)
	cleaned = multiBlankLinePattern.ReplaceAllString(cleaned, "\n\n")
	return strings.TrimSpace(cleaned)
}

func collectFunctionSpecs(tools []ToolSpec, functions []FunctionSpec) []FunctionSpec {
	specs := make([]FunctionSpec, 0, len(tools)+len(functions))
	for _, tool := range tools {
		if tool.Type != "" && !strings.EqualFold(tool.Type, "function") {
			continue
		}
		if strings.TrimSpace(tool.Function.Name) == "" {
			continue
		}
		specs = append(specs, tool.Function)
	}
	for _, fn := range functions {
		if strings.TrimSpace(fn.Name) == "" {
			continue
		}
		specs = append(specs, fn)
	}
	return specs
}

func extractToolCallsBody(content string) string {
	startTag := "<tool_calls>"
	endTag := "</tool_calls>"
	start := strings.Index(content, startTag)
	end := strings.LastIndex(content, endTag)
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return content[start+len(startTag) : end]
}

func parseToolArgumentsXML(body string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "{}"
	}

	// 兼容旧格式：<tool_name>{"arg":"v"}</tool_name>
	if json.Valid([]byte(trimmed)) {
		var any interface{}
		if err := json.Unmarshal([]byte(trimmed), &any); err == nil {
			if _, ok := any.(map[string]interface{}); ok {
				data, _ := json.Marshal(any)
				return string(data)
			}
		}
	}

	args := map[string]interface{}{}
	matches := xmlGenericBlockPattern.FindAllStringSubmatch(trimmed, -1)
	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		openTag := strings.TrimSpace(m[1])
		closeTag := strings.TrimSpace(m[3])
		if openTag == "" || closeTag == "" || openTag != closeTag {
			continue
		}
		key := openTag
		if key == "" {
			continue
		}
		rawVal := decodeXMLParamValue(m[2])
		args[key] = normalizeXMLValue(rawVal)
	}

	if len(args) == 0 {
		return "{}"
	}
	data, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func decodeXMLParamValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "<![CDATA[") && strings.HasSuffix(trimmed, "]]>") {
		inner := strings.TrimPrefix(trimmed, "<![CDATA[")
		inner = strings.TrimSuffix(inner, "]]>")
		inner = strings.ReplaceAll(inner, "]]]]><![CDATA[>", "]]>")
		return inner
	}
	return strings.TrimSpace(trimmed)
}

func normalizeXMLValue(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if json.Valid([]byte(raw)) {
		var any interface{}
		if err := json.Unmarshal([]byte(raw), &any); err == nil {
			return any
		}
	}
	return raw
}

func buildToolChoiceInstruction(toolChoice interface{}, functionCall interface{}) string {
	if forcedName, forced := extractForcedToolName(toolChoice, functionCall); forced {
		if forcedName != "" {
			return fmt.Sprintf("CRITICAL: The client requires a tool call now. You MUST call <%s> in your next response.", sanitizeXMLTagName(forcedName))
		}
		return "CRITICAL: The client requires a tool call now. Your next response MUST be an XML tool call."
	}
	return ""
}

func extractForcedToolName(toolChoice interface{}, functionCall interface{}) (string, bool) {
	if name, forced := parseForcedNameFromChoice(toolChoice); forced {
		return name, true
	}
	if name, forced := parseForcedNameFromChoice(functionCall); forced {
		return name, true
	}
	return "", false
}

func parseForcedNameFromChoice(raw interface{}) (string, bool) {
	if raw == nil {
		return "", false
	}

	switch v := raw.(type) {
	case string:
		choice := strings.ToLower(strings.TrimSpace(v))
		switch choice {
		case "required", "any":
			return "", true
		case "auto", "none", "":
			return "", false
		default:
			return strings.TrimSpace(v), true
		}

	case map[string]interface{}:
		if rawType, ok := v["type"].(string); ok && strings.EqualFold(strings.TrimSpace(rawType), "function") {
			if fn, ok := v["function"].(map[string]interface{}); ok {
				if name, ok := fn["name"].(string); ok && strings.TrimSpace(name) != "" {
					return strings.TrimSpace(name), true
				}
			}
			return "", true
		}
		if name, ok := v["name"].(string); ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name), true
		}
		if rawValue, ok := v["value"].(string); ok {
			choice := strings.ToLower(strings.TrimSpace(rawValue))
			if choice == "required" || choice == "any" {
				return "", true
			}
		}
		return "", false
	}

	return "", false
}
