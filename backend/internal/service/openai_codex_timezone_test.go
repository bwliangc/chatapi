package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestRewriteCodexTimezoneInBody(t *testing.T) {
	targetTimezone := timezone.Name()
	if targetTimezone == "Local" || targetTimezone == "" {
		targetTimezone = timezone.Location().String()
	}
	targetDate := timezone.Now().Format("2006-01-02")

	input := []byte(`{"input":[{"type":"input_text","text":"before <environment_context>\n<current_date>2020-01-01</current_date>\n<timezone>America/Los_Angeles</timezone>\n</environment_context> after"}],"n":1}`)
	rewritten, changed, err := rewriteCodexTimezoneInBody(input, true)
	require.NoError(t, err)
	require.True(t, changed)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(rewritten, &decoded))
	inputItems := decoded["input"].([]any)
	text := inputItems[0].(map[string]any)["text"].(string)
	require.Contains(t, text, "<timezone>"+targetTimezone+"</timezone>")
	require.Contains(t, text, "<current_date>"+targetDate+"</current_date>")
	require.Contains(t, string(rewritten), "\"n\":1")

	again, changed, err := rewriteCodexTimezoneInBody(rewritten, true)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, rewritten, again)
}

func TestRewriteCodexTimezoneInBodyWithConfiguredTimezone(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)
	body := []byte(`{"text":"<environment_context><timezone>Asia/Shanghai</timezone><current_date>2000-01-01</current_date></environment_context>"}`)
	rewritten, changed, err := rewriteCodexTimezoneInBodyWithTimezone(body, true, "America/Los_Angeles")
	require.NoError(t, err)
	require.True(t, changed)
	var decoded map[string]string
	require.NoError(t, json.Unmarshal(rewritten, &decoded))
	expectedDate := time.Now().In(location).Format("2006-01-02")
	require.Contains(t, decoded["text"], "<timezone>America/Los_Angeles</timezone>")
	require.Contains(t, decoded["text"], "<current_date>"+expectedDate+"</current_date>")
}

func TestRewriteCodexTimezoneInBodyRejectsInvalidConfiguredTimezone(t *testing.T) {
	body := []byte(`{"text":"<environment_context><timezone>UTC</timezone></environment_context>"}`)
	_, changed, err := rewriteCodexTimezoneInBodyWithTimezone(body, true, "Invalid/Timezone")
	require.Error(t, err)
	require.False(t, changed)
}

func TestRewriteCodexTimezoneInBodyBoundaries(t *testing.T) {
	targetTimezone := timezone.Name()
	if targetTimezone == "Local" || targetTimezone == "" {
		targetTimezone = timezone.Location().String()
	}
	targetDate := timezone.Now().Format("2006-01-02")

	withoutContext := []byte(`{"text":"<timezone>America/Los_Angeles</timezone>"}`)
	rewritten, changed, err := rewriteCodexTimezoneInBody(withoutContext, true)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, withoutContext, rewritten)

	closedContext := []byte(`{"a":"<environment_context><timezone>America/Los_Angeles</timezone><current_date>2020-01-01</current_date></environment_context>","b":"<environment_context><timezone>UTC</timezone><current_date>2020-01-02</current_date></environment_context>"}`)
	rewritten, changed, err = rewriteCodexTimezoneInBody(closedContext, true)
	require.NoError(t, err)
	require.True(t, changed)
	var decoded map[string]string
	require.NoError(t, json.Unmarshal(rewritten, &decoded))
	require.Contains(t, decoded["a"], "<environment_context>")
	require.Contains(t, decoded["b"], "<environment_context>")
	require.NotContains(t, decoded["a"], "America/Los_Angeles")
	require.NotContains(t, decoded["b"], "2020-01-02")

	unchanged, changed, err := rewriteCodexTimezoneInBody(closedContext, false)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, closedContext, unchanged)

	repeatedTag := `prefix <environment_context><timezone>` + targetTimezone + `</timezone> and <timezone>America/Los_Angeles</timezone><current_date>` + targetDate + `</current_date></environment_context> suffix`
	rewrittenText, changed := rewriteCodexTimezoneText(repeatedTag, targetTimezone, targetDate)
	require.True(t, changed)
	require.Contains(t, rewrittenText, "prefix <environment_context>")
	require.NotContains(t, rewrittenText, "America/Los_Angeles")

	mixedBlocks := "keep <environment_context><timezone>" + targetTimezone + "</timezone><current_date>" + targetDate + "</current_date></environment_context> then <environment_context><timezone>America/Los_Angeles</timezone><current_date>2020-01-01</current_date></environment_context>"
	rewrittenText, changed = rewriteCodexTimezoneText(mixedBlocks, targetTimezone, targetDate)
	require.True(t, changed)
	require.Contains(t, rewrittenText, "keep <environment_context>")
	require.Contains(t, rewrittenText, "</environment_context> then <environment_context>")
	require.NotContains(t, rewrittenText, "America/Los_Angeles")
}
