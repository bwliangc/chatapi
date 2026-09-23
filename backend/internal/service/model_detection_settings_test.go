//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestModelDetectionPublicSettingOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "true"} {
		repo := &settingPublicRepoStub{values: map[string]string{SettingKeyModelDetectionEnabled: value}}
		svc := NewSettingService(repo, &config.Config{})
		settings, err := svc.GetPublicSettings(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if settings.ModelDetectionEnabled != (value == "true") {
			t.Fatalf("opt-in mismatch for %q", value)
		}
		raw, _ := json.Marshal(settings)
		var decoded map[string]any
		_ = json.Unmarshal(raw, &decoded)
		if decoded["model_detection_enabled"] != (value == "true") {
			t.Fatal("public field missing")
		}
	}
}
