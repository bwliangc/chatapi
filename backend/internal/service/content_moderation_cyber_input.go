package service

import (
	"strings"
	"unicode/utf8"

	"github.com/tidwall/gjson"
)

const maxCyberPolicyExcerptRunes = 4000

// A post-upstream block may occur during a tool turn. Include the latest user
// input still present in the request, even when assistant/tool messages follow it.
// Keep the pre-request moderation extractor's tool-turn skipping unchanged.
// This preview is only for the audit log; the email attaches the original body.
func extractCyberPolicyInputExcerpt(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	payload := gjson.ParseBytes(body)
	var parts, images []string
	input := payload.Get("input")
	if input.IsArray() {
		items := input.Array()
		for i := len(items) - 1; i >= 0; i-- {
			if isResponsesUserTextItem(items[i]) {
				input = items[i]
				break
			}
		}
	}
	collectLastResponsesInput(input, &parts, &images)
	if len(parts) == 0 {
		messages := payload.Get("messages").Array()
		for i := len(messages) - 1; i >= 0; i-- {
			if !strings.EqualFold(strings.TrimSpace(messages[i].Get("role").String()), "user") {
				continue
			}
			collectAnthropicUserContentValue(messages[i].Get("content"), &parts, &images)
			if len(parts) > 0 {
				break
			}
		}
	}
	text := redactContentModerationSecrets(strings.Join(parts, "\n"))
	if utf8.RuneCountInString(text) > maxCyberPolicyExcerptRunes {
		return trimRunes(text, maxCyberPolicyExcerptRunes) + "\n…（内容已截断 / content truncated）"
	}
	return text
}
