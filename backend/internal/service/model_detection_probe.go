package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/tidwall/gjson"
)

const modelDetectionOutputLimit = 128 * 1024

// Only local account transport helpers are used: no gateway account selection.
func (s *AccountTestService) probeModelIdentity(ctx context.Context, account *Account, model, prompt string) (string, string, error) {
	model = account.GetMappedModel(model)
	protocol := "responses"
	payload := map[string]any{}
	headers := http.Header{"Content-Type": {"application/json"}, "Accept": {"text/event-stream"}}
	apiURL := ""
	if account.Platform == PlatformAnthropic {
		protocol = "anthropic"
		payload = map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": prompt}}, "stream": true, "max_tokens": 4096, "temperature": 1}
		headers.Set("anthropic-version", "2023-06-01")
		token := account.GetCredential("api_key")
		if account.IsOAuth() {
			token = account.GetCredential("access_token")
			if s.claudeTokenProvider != nil {
				var err error
				token, err = s.claudeTokenProvider.GetAccessToken(ctx, account)
				if err != nil {
					return "", "", errors.New("刷新账号认证失败")
				}
			}
			apiURL = testClaudeAPIURL
			for key, value := range claude.DefaultHeaders() {
				headers.Set(key, value)
			}
			headers.Set("anthropic-beta", claude.DefaultBetaHeader)
			headers.Set("Authorization", "Bearer "+token)
			// OAuth requires the Claude Code identity prefix, matching account tests.
			payload["system"] = []map[string]string{{"type": "text", "text": claudeCodeSystemPrompt}}
			session, err := generateSessionString()
			if err != nil {
				return "", "", errors.New("创建探测会话失败")
			}
			payload["metadata"] = map[string]string{"user_id": session}
		} else {
			base := account.GetBaseURL()
			if base == "" {
				base = "https://api.anthropic.com"
			}
			normalized, err := s.validateUpstreamBaseURL(base)
			if err != nil {
				return "", "", errors.New("账号地址配置无效")
			}
			apiURL = strings.TrimSuffix(strings.TrimSuffix(normalized, "/"), "/v1") + "/v1/messages"
			setAnthropicAPIKeyAuthHeader(headers, account, token, base)
		}
		if token == "" {
			return "", "", errors.New("账号缺少认证凭据")
		}
	} else {
		oauth := account.IsOAuth()
		token := account.GetOpenAIProtocolAPIKey()
		if oauth {
			model = normalizeOpenAIModelForUpstream(account, model)
			token = account.GetOpenAIAccessToken()
			apiURL = chatgptCodexAPIURL
			headers.Set("OpenAI-Beta", "responses=experimental")
			canonical := resolveCodexOutboundIdentity("")
			headers.Set("Originator", canonical.originator)
			headers.Set("User-Agent", canonical.userAgent)
			setOpenAIChatGPTAccountHeaders(headers, account)
			enforceCodexIdentityHeadersWithUA(headers, account.GetOpenAIUserAgent())
		} else {
			base := account.GetOpenAIBaseURL()
			if base == "" {
				base = "https://api.openai.com"
			}
			normalized, err := s.validateUpstreamBaseURL(base)
			if err != nil {
				return "", "", errors.New("账号地址配置无效")
			}
			apiURL = buildOpenAIResponsesURLForPlatform(account.Platform, normalized)
			if !openai_compat.ShouldUseResponsesAPI(account.Extra) {
				protocol = "chat"
				apiURL = buildOpenAIChatCompletionsURL(normalized)
			}
		}
		if account.IsOpenAIAgentIdentity() {
			auth, err := buildAgentIdentityAuthenticationHeaders(ctx, s.accountRepo, s.agentIdentityWS, &s.agentIdentityTaskMu, account)
			if err != nil {
				return "", "", errors.New("创建账号认证失败")
			}
			for key, values := range auth {
				headers[key] = values
			}
		} else {
			if token == "" {
				return "", "", errors.New("账号缺少认证凭据")
			}
			headers.Set("Authorization", "Bearer "+token)
		}
		if protocol == "chat" {
			payload = map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": prompt}}, "stream": true, "max_completion_tokens": 4096}
		} else {
			payload = map[string]any{"model": model, "input": []map[string]string{{"role": "user", "content": prompt}}, "instructions": "", "stream": true, "tools": []any{}, "store": false}
			if oauth {
				payload["instructions"] = openai.DefaultInstructions
			}
			// Codex OAuth rejects max_output_tokens; bound its response bytes and time.
			if !oauth {
				payload["max_output_tokens"] = 4096
				applyOpenAICodexProbeHeaders(headers)
			}
		}
	}
	// No tools are supplied in any protocol; each call has one fresh user message.
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", "", errors.New("构建探测请求失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(raw))
	if err != nil {
		return "", "", errors.New("构建探测请求失败")
	}
	req.Header = headers
	account.ApplyHeaderOverrides(req.Header)
	proxyURL := ""
	if account.Proxy != nil && account.ProxyID != nil {
		proxyURL = account.Proxy.URL()
	}
	var resp *http.Response
	if account.Platform == PlatformOpenAI {
		req = req.WithContext(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileOpenAI))
		if account.IsOAuth() {
			req.Host = "chatgpt.com"
		}
		resp, err = s.doOpenAIAccountTestUpstream(req, proxyURL, account, true)
	} else {
		resp, err = s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	}
	if err != nil {
		return "", "", errors.New("上游请求失败，请检查账号认证、代理或网络")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("上游返回 HTTP %d", resp.StatusCode)
	}
	return parseModelDetectionStream(resp.Body, protocol)
}

// Require protocol-level normal termination, reject truncated/tool output and
// bound both total upstream bytes and accumulated text. Never log raw bodies.
func parseModelDetectionStream(body io.Reader, protocol string) (string, string, error) {
	scanner := bufio.NewScanner(io.LimitReader(body, 2*1024*1024+1))
	scanner.Buffer(make([]byte, 4096), 256*1024)
	var output strings.Builder
	actual, finish := "", ""
	completed := false
	var data []string
	consume := func() error {
		if len(data) == 0 {
			return nil
		}
		raw := strings.Join(data, "\n")
		data = nil
		if raw == "[DONE]" {
			if protocol == "chat" && finish == "stop" {
				completed = true
			}
			return nil
		}
		if !gjson.Valid(raw) {
			return errors.New("上游返回无效事件")
		}
		v := gjson.Parse(raw)
		text := ""
		switch protocol {
		case "responses":
			if m := v.Get("response.model").String(); m != "" {
				actual = m
			}
			switch v.Get("type").String() {
			case "response.output_text.delta":
				if v.Get("delta").Type != gjson.String {
					return errors.New("上游返回非文本内容")
				}
				text = v.Get("delta").String()
			case "response.output_item.added", "response.output_item.done":
				kind := v.Get("item.type").String()
				if kind != "message" && kind != "reasoning" {
					return errors.New("上游返回工具或非文本输出")
				}
			case "response.completed":
				if v.Get("response.status").String() != "completed" {
					return errors.New("上游回答未正常完成")
				}
				for _, item := range v.Get("response.output").Array() {
					kind := item.Get("type").String()
					if kind != "message" && kind != "reasoning" {
						return errors.New("上游返回工具或非文本输出")
					}
				}
				completed = true
			case "response.failed", "response.incomplete", "error":
				return errors.New("上游回答失败或被截断")
			}
		case "anthropic":
			if m := v.Get("message.model").String(); m != "" {
				actual = m
			}
			switch v.Get("type").String() {
			case "content_block_start":
				kind := v.Get("content_block.type").String()
				if kind != "text" && kind != "thinking" && kind != "redacted_thinking" {
					return errors.New("上游返回工具或非文本输出")
				}
				text = v.Get("content_block.text").String()
			case "content_block_delta":
				if v.Get("delta.type").String() == "text_delta" {
					if v.Get("delta.text").Type != gjson.String {
						return errors.New("上游返回非文本内容")
					}
					text = v.Get("delta.text").String()
				}
			case "message_delta":
				finish = v.Get("delta.stop_reason").String()
			case "message_stop":
				completed = finish == "end_turn"
			case "error":
				return errors.New("上游回答失败")
			}
		case "chat":
			if m := v.Get("model").String(); m != "" {
				actual = m
			}
			choices := v.Get("choices").Array()
			for _, choice := range choices {
				if choice.Get("index").Int() != 0 {
					continue
				}
				if len(choice.Get("delta.tool_calls").Array()) > 0 || choice.Get("delta.function_call").Exists() {
					return errors.New("上游返回工具输出")
				}
				content := choice.Get("delta.content")
				if content.Type != gjson.Null && content.Type != gjson.String {
					return errors.New("上游返回非文本内容")
				}
				text += content.String()
				if f := choice.Get("finish_reason").String(); f != "" {
					finish = f
					completed = f == "stop"
				}
			}
		default:
			return errors.New("不支持的响应协议")
		}
		if output.Len()+len(text) > modelDetectionOutputLimit {
			return errors.New("上游回答超过长度限制")
		}
		output.WriteString(text)
		if len(actual) > 160 {
			return errors.New("上游模型名称无效")
		}
		return nil
	}
	bytesRead := 0
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		bytesRead += len(line) + 1
		if bytesRead > 2*1024*1024 {
			return "", "", errors.New("上游事件超过长度限制")
		}
		if line == "" {
			if err := consume(); err != nil {
				return "", "", err
			}
			if completed {
				if output.Len() == 0 {
					return "", "", errors.New("上游没有返回文本")
				}
				return output.String(), actual, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", errors.New("读取上游回答失败")
	}
	if err := consume(); err != nil {
		return "", "", err
	}
	if !completed || output.Len() == 0 {
		return "", "", errors.New("上游回答未完整结束")
	}
	return output.String(), actual, nil
}
