package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

func (s *OpenAIGatewayService) rewriteCodexTimezoneIfEnabled(ctx context.Context, account *Account, body []byte) ([]byte, error) {
	if s == nil || s.settingService == nil || account == nil || !account.UsesOpenAICodexProtocol() {
		return body, nil
	}
	rewritten, _, err := rewriteCodexTimezoneInBodyWithTimezone(body, s.settingService.IsCodexTimezoneRewriteEnabled(ctx), s.settingService.GetCodexTimezone(ctx))
	return rewritten, err
}

// rewriteCodexTimezoneInBody rewrites the timezone and current date inside
// Codex's <environment_context> prompt block. It walks all JSON strings so it
// also handles Responses input arrays and nested message/tool payloads.
// Requests without an environment_context block are returned byte-for-byte
// unchanged.
func rewriteCodexTimezoneInBody(body []byte, enabled bool) ([]byte, bool, error) {
	return rewriteCodexTimezoneInBodyWithTimezone(body, enabled, "")
}

func rewriteCodexTimezoneInBodyWithTimezone(body []byte, enabled bool, configuredTimezone string) ([]byte, bool, error) {
	if !enabled || len(body) == 0 {
		return body, false, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return body, false, fmt.Errorf("decode Codex request for timezone rewrite: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return body, false, fmt.Errorf("decode Codex request for timezone rewrite: multiple JSON values")
		}
		return body, false, fmt.Errorf("decode Codex request trailing data for timezone rewrite: %w", err)
	}

	targetLocation := timezone.Location()
	targetTimezone := strings.TrimSpace(configuredTimezone)
	if targetTimezone != "" {
		location, err := time.LoadLocation(targetTimezone)
		if err != nil {
			return body, false, fmt.Errorf("invalid Codex rewrite timezone %q: %w", targetTimezone, err)
		}
		targetLocation = location
	} else {
		targetTimezone = strings.TrimSpace(timezone.Name())
		if targetTimezone == "" || targetTimezone == "Local" {
			targetTimezone = targetLocation.String()
		}
	}
	if targetTimezone == "" || targetTimezone == "Local" {
		targetTimezone = "UTC"
	}
	targetDate := time.Now().In(targetLocation).Format("2006-01-02")

	changed := rewriteCodexTimezoneValue(&value, targetTimezone, targetDate)
	if !changed {
		return body, false, nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return body, false, fmt.Errorf("encode Codex request after timezone rewrite: %w", err)
	}
	return encoded, true, nil
}

func rewriteCodexTimezoneValue(value *any, targetTimezone, targetDate string) bool {
	switch current := (*value).(type) {
	case string:
		rewritten, changed := rewriteCodexTimezoneText(current, targetTimezone, targetDate)
		if changed {
			*value = rewritten
		}
		return changed
	case []any:
		changed := false
		for i := range current {
			item := any(current[i])
			if rewriteCodexTimezoneValue(&item, targetTimezone, targetDate) {
				current[i] = item
				changed = true
			}
		}
		return changed
	case map[string]any:
		changed := false
		for key, item := range current {
			if rewriteCodexTimezoneValue(&item, targetTimezone, targetDate) {
				current[key] = item
				changed = true
			}
		}
		return changed
	default:
		return false
	}
}

func rewriteCodexTimezoneText(text, targetTimezone, targetDate string) (string, bool) {
	const (
		openTag  = "<environment_context>"
		closeTag = "</environment_context>"
	)

	var output strings.Builder
	scanFrom := 0
	copyFrom := 0
	changed := false
	for scanFrom < len(text) {
		openOffset := strings.Index(text[scanFrom:], openTag)
		if openOffset < 0 {
			break
		}
		openOffset += scanFrom
		contentStart := openOffset + len(openTag)
		closeOffset := strings.Index(text[contentStart:], closeTag)
		if closeOffset < 0 {
			break
		}
		closeOffset += contentStart

		block := text[contentStart:closeOffset]
		rewrittenBlock, blockChanged := rewriteCodexTimezoneTags(block, targetTimezone, targetDate)
		if blockChanged {
			if !changed {
				output.Grow(len(text))
			}
			output.WriteString(text[copyFrom:contentStart])
			output.WriteString(rewrittenBlock)
			copyFrom = closeOffset
			changed = true
		}
		scanFrom = closeOffset + len(closeTag)
	}
	if !changed {
		return text, false
	}
	output.WriteString(text[copyFrom:])
	return output.String(), true
}

func rewriteCodexTimezoneTags(block, targetTimezone, targetDate string) (string, bool) {
	rewritten, timezoneChanged := replaceCodexTimezoneTag(block, "timezone", targetTimezone)
	rewritten, dateChanged := replaceCodexTimezoneTag(rewritten, "current_date", targetDate)
	return rewritten, timezoneChanged || dateChanged
}

func replaceCodexTimezoneTag(text, tag, replacement string) (string, bool) {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	var output strings.Builder
	scanFrom := 0
	copyFrom := 0
	changed := false
	for scanFrom < len(text) {
		openOffset := strings.Index(text[scanFrom:], openTag)
		if openOffset < 0 {
			break
		}
		openOffset += scanFrom
		valueStart := openOffset + len(openTag)
		closeOffset := strings.Index(text[valueStart:], closeTag)
		if closeOffset < 0 {
			break
		}
		closeOffset += valueStart
		if text[valueStart:closeOffset] == replacement {
			scanFrom = closeOffset + len(closeTag)
			continue
		}
		if !changed {
			output.Grow(len(text))
		}
		output.WriteString(text[copyFrom:valueStart])
		output.WriteString(replacement)
		copyFrom = closeOffset
		scanFrom = closeOffset + len(closeTag)
		changed = true
	}
	if !changed {
		return text, false
	}
	output.WriteString(text[copyFrom:])
	return output.String(), true
}
