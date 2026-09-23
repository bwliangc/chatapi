package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/modeltrace"
)

type detectionBankRepo struct {
	SettingRepository
	mu        sync.Mutex
	raw       string
	failWrite bool
}

func (r *detectionBankRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if key == SettingKeyModelDetectionEnabled {
		return "true", nil
	}
	if r.raw == "" {
		return "", ErrSettingNotFound
	}
	return r.raw, nil
}
func (r *detectionBankRepo) CompareAndSwapValue(_ context.Context, _ string, expected, next string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failWrite {
		return false, errors.New("db failure")
	}
	if r.raw != expected {
		return false, nil
	}
	r.raw = next
	return true, nil
}

type detectionBankTransport func(*http.Request) (*http.Response, error)

func (f detectionBankTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func bankTestData(t *testing.T, date string) []byte {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(modeltrace.EmbeddedData(), &v); err != nil {
		t.Fatal(err)
	}
	v["built_at"] = date
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func bankTestManager(t *testing.T, repo *detectionBankRepo) (*modelDetectionBankManager, []byte) {
	t.Helper()
	code, err := os.ReadFile("../../../third_party/modeltrace/fingerprint-core.mjs")
	if err != nil {
		t.Fatal(err)
	}
	candidate := bankTestData(t, "2026-09-24T00:00:00Z")
	manager := newModelDetectionBankManager(repo)
	manager.client.Transport = detectionBankTransport(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "" {
			t.Fatal("unexpected credentials sent to update source")
		}
		var raw []byte
		switch {
		case req.URL.Host == "api.github.com":
			raw = []byte(`{"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
		case strings.HasSuffix(req.URL.Path, "fingerprint-core.js"):
			raw = code
		case strings.HasSuffix(req.URL.Path, "unified_bank.json"):
			raw = candidate
		default:
			t.Fatalf("unexpected URL %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(raw)))}, nil
	})
	return manager, candidate
}
func TestModelDetectionBankUpdatePersistenceRollback(t *testing.T) {
	ctx := context.Background()
	repo := &detectionBankRepo{}
	manager, candidate := bankTestManager(t, repo)
	check, err := manager.check(ctx)
	if err != nil || !check.Compatible || !check.UpdateAvailable {
		t.Fatalf("check %+v %v", check, err)
	}
	_, original, _, _ := manager.load(ctx)
	pinned, err := manager.parse(original)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := manager.update(ctx, check.Remote.Commit, check.Status.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Current.SHA256 != modelDetectionDigest(candidate) || updated.Previous == nil || updated.Source != "database" {
		t.Fatalf("bad update %+v", updated)
	}
	// A fresh manager (restart or another node) loads the identical saved version.
	restarted := newModelDetectionBankManager(repo)
	saved, _, _, err := restarted.load(ctx)
	if err != nil || saved.Current.SHA256 != updated.Current.SHA256 {
		t.Fatalf("not persisted %+v %v", saved, err)
	}
	service := &AccountTestService{settingService: NewSettingService(repo, nil)}
	bank, err := service.CurrentModelDetectionBank(ctx)
	if err != nil || bank.SHA256 != updated.Current.SHA256 {
		t.Fatal("live detection did not load new bank", err)
	}
	if pinned.SHA256 != check.Status.Current.SHA256 || pinned.BuiltAt == bank.BuiltAt {
		t.Fatal("in-flight snapshot changed")
	}
	if _, err = manager.update(ctx, check.Remote.Commit, check.Status.Revision); !errors.Is(err, ErrModelDetectionBankConflict) {
		t.Fatal("stale update accepted", err)
	}
	rollback, err := restarted.rollback(ctx, saved.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if rollback.Current.SHA256 != pinned.SHA256 || rollback.Previous != nil {
		t.Fatalf("bad rollback %+v", rollback)
	}
	if _, err = restarted.rollback(ctx, rollback.Revision); err == nil {
		t.Fatal("repeated rollback accepted")
	}
}
func TestModelDetectionBankRejectsAlgorithmAndBadData(t *testing.T) {
	for _, kind := range []string{"algorithm", "invalid-bank", "oversized", "network", "redirect", "rate-limit"} {
		t.Run(kind, func(t *testing.T) {
			repo := &detectionBankRepo{}
			manager, _ := bankTestManager(t, repo)
			transport := manager.client.Transport
			manager.client.Transport = detectionBankTransport(func(req *http.Request) (*http.Response, error) {
				if kind == "network" {
					return nil, errors.New("secret-proxy-password")
				}
				if kind == "redirect" {
					return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://unexpected.example"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
				}
				if kind == "rate-limit" {
					return &http.Response{StatusCode: 429, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("private-error"))}, nil
				}
				if strings.HasSuffix(req.URL.Path, "fingerprint-core.js") && kind == "algorithm" {
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("changed scorer")), Header: http.Header{}}, nil
				}
				if strings.HasSuffix(req.URL.Path, "unified_bank.json") {
					raw := "{}"
					if kind == "oversized" {
						raw = strings.Repeat(" ", modeltrace.MaxBankBytes+1)
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(raw)), Header: http.Header{}}, nil
				}
				return transport.RoundTrip(req)
			})
			status, _, _, _ := manager.load(context.Background())
			_, err := manager.update(context.Background(), strings.Repeat("a", 40), status.Revision)
			if err == nil || strings.Contains(err.Error(), "secret") || repo.raw != "" {
				t.Fatalf("invalid update affected stored state: %v", err)
			}
			if kind == "algorithm" {
				check, err := manager.check(context.Background())
				if err != nil || check.Compatible || check.Message == "" {
					t.Fatalf("missing upgrade notice: %+v %v", check, err)
				}
			}
		})
	}
}
func TestModelDetectionBankFailedSaveAndConcurrentCAS(t *testing.T) {
	repo := &detectionBankRepo{}
	manager, _ := bankTestManager(t, repo)
	ctx := context.Background()
	status, _, _, _ := manager.load(ctx)
	repo.failWrite = true
	if _, err := manager.update(ctx, strings.Repeat("a", 40), status.Revision); err == nil || repo.raw != "" {
		t.Fatal("save failure switched banks")
	}
	repo.failWrite = false
	// A concurrent writer wins after validation but before this update commits.
	originalTransport := manager.client.Transport
	manager.client.Transport = detectionBankTransport(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "unified_bank.json") {
			repo.mu.Lock()
			repo.raw = `{"current":{}}`
			repo.mu.Unlock()
		}
		return originalTransport.RoundTrip(req)
	})
	if _, err := manager.update(ctx, strings.Repeat("a", 40), status.Revision); !errors.Is(err, ErrModelDetectionBankConflict) {
		t.Fatal("lost concurrent update", err)
	}
}
func TestModelDetectionBankFallbackAfterAlgorithmUpgrade(t *testing.T) {
	repo := &detectionBankRepo{}
	snapshot := embeddedModelDetectionSnapshot()
	snapshot.AlgorithmSHA256 = strings.Repeat("0", 64)
	raw, _ := json.Marshal(modelDetectionBankState{Current: snapshot})
	repo.raw = string(raw)
	manager := newModelDetectionBankManager(repo)
	status, _, _, err := manager.load(context.Background())
	if err != nil || status.Source != "embedded" || status.Warning == "" || status.Previous != nil {
		t.Fatalf("incompatible data used: %+v %v", status, err)
	}
}

func TestModelDetectionBankRecoversMalformedState(t *testing.T) {
	repo := &detectionBankRepo{raw: `{"current":`}
	manager, _ := bankTestManager(t, repo)
	ctx := context.Background()
	status, _, _, err := manager.load(ctx)
	if err != nil || status.Source != "embedded" || status.Warning == "" {
		t.Fatalf("corrupt state did not fall back: %+v %v", status, err)
	}
	updated, err := manager.update(ctx, strings.Repeat("a", 40), status.Revision)
	if err != nil || updated.Source != "database" || updated.Warning != "" {
		t.Fatalf("could not replace corrupt state: %+v %v", updated, err)
	}
}
