package service

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestExtractCyberPolicyInputExcerpt(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "responses string preserves line breaks",
			body: `{"instructions":"private system text","input":"first line\n第二行"}`,
			want: "first line\n第二行",
		},
		{
			name: "responses tool turn retains latest user input",
			body: `{"input":[{"role":"user","content":"old request"},{"role":"user","content":[{"type":"input_text","text":"latest request"},{"type":"input_image","image_url":"data:image/png;base64,private-image"}]},{"role":"assistant","content":"private reasoning"},{"type":"function_call_output","output":"private tool output"}]}`,
			want: "latest request",
		},
		{
			name: "responses text item",
			body: `{"input":[{"type":"input_text","text":"request without role"}]}`,
			want: "request without role",
		},
		{
			name: "chat tool turn retains latest user input",
			body: `{"messages":[{"role":"system","content":"private system text"},{"role":"user","content":"old request"},{"role":"user","content":"latest request"},{"role":"assistant","content":"private reasoning"},{"role":"tool","content":"private tool output"}]}`,
			want: "latest request",
		},
		{
			name: "anthropic tool results and system reminders are omitted",
			body: `{"messages":[{"role":"user","content":[{"type":"text","text":"<system-reminder>private system text</system-reminder>"},{"type":"text","text":"latest request"}]},{"role":"assistant","content":[{"type":"tool_use","name":"bash","input":{"command":"private command"}}]},{"role":"user","content":[{"type":"tool_result","content":"private tool output"}]}]}`,
			want: "latest request",
		},
		{
			name: "websocket response create",
			body: `{"type":"response.create","input":[{"role":"user","content":"websocket request"}]}`,
			want: "websocket request",
		},
		{
			name: "secret redaction",
			body: `{"input":"check api_key=sk-1234567890abcdefghijklmnop\nBearer abcdefghijklmnopqrstuvwxyz"}`,
			want: "check api_key=[已脱敏]\nBearer [已脱敏]",
		},
		{
			name: "tool-only continuation",
			body: `{"input":[{"type":"function_call_output","output":"private tool output"}]}`,
		},
		{
			name: "image-only input",
			body: `{"input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,private-image"}]}]}`,
		},
		{name: "missing body"},
		{name: "invalid body", body: `{"input":`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, extractCyberPolicyInputExcerpt([]byte(tt.body)))
		})
	}
}

func TestExtractCyberPolicyInputExcerpt_TruncatesAfterRedaction(t *testing.T) {
	prefix := strings.Repeat("文", maxCyberPolicyExcerptRunes-4)
	body, err := json.Marshal(map[string]string{
		"input": prefix + " sk-1234567890abcdefghijklmnop 后续内容",
	})
	require.NoError(t, err)
	excerpt := extractCyberPolicyInputExcerpt(body)
	require.True(t, utf8.ValidString(excerpt))
	require.True(t, strings.HasPrefix(excerpt, prefix))
	require.Contains(t, excerpt, "内容已截断")
	require.NotContains(t, excerpt, "sk-")
	require.NotContains(t, excerpt, "后续内容")

	body, err = json.Marshal(map[string]string{"input": strings.Repeat("文", maxCyberPolicyExcerptRunes)})
	require.NoError(t, err)
	excerpt = extractCyberPolicyInputExcerpt(body)
	require.Equal(t, maxCyberPolicyExcerptRunes, utf8.RuneCountInString(excerpt))
	require.NotContains(t, excerpt, "内容已截断")
}
