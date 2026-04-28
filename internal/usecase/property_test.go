package usecase

import (
	"fmt"
	"testing"
	"time"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
	"pgregory.net/rapid"
)

func TestPropertyAgentSetKeepsBackupHistoryBounded(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		setCount := rapid.IntRange(1, 12).Draw(t, "setCount")

		repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{
			"relay-a": {
				Name:      "relay-a",
				BaseURL:   "https://relay.example/v1",
				APIKey:    "sk-a",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		}}
		state := &fakeStateRepo{}
		codexBase := newFakeAgentSyncer("/tmp/codex/config.toml")
		codexBase.snapshotContent = []byte("before")
		codex := &fakeCodexSyncer{codexBase}
		svc := NewProfileService(repo, state, codex, nil, nil)

		for i := 0; i < setCount; i++ {
			codex.snapshotContent = []byte(fmt.Sprintf("before-%d", i))
			if _, err := svc.AgentSet(domainprofile.AgentCodex, "relay-a"); err != nil {
				t.Fatalf("AgentSet() error = %v", err)
			}
		}

		backups := state.state.Codex.Backups
		if len(backups) == 0 {
			t.Fatalf("expected backups after AgentSet")
		}
		if len(backups) > backupHistoryLimit {
			t.Fatalf("len(backups) = %d, want <= %d", len(backups), backupHistoryLimit)
		}
		if state.state.Codex.LastBackupID != backups[0].ID {
			t.Fatalf("LastBackupID = %q, want newest backup %q", state.state.Codex.LastBackupID, backups[0].ID)
		}
	})
}

func TestPropertyAddAllowsDuplicateNormalizedBaseURL(t *testing.T) {
	validHost := rapid.StringMatching(`[A-Za-z0-9-]{1,12}`).Filter(func(s string) bool {
		return s != ""
	})

	rapid.Check(t, func(t *rapid.T) {
		host := validHost.Draw(t, "host")
		schemeA := rapid.SampledFrom([]string{"https", "HTTPS"}).Draw(t, "schemeA")
		schemeB := rapid.SampledFrom([]string{"https", "HTTPS"}).Draw(t, "schemeB")
		pathA := rapid.SampledFrom([]string{"/v1", "/v1/", "/v1///"}).Draw(t, "pathA")
		pathB := rapid.SampledFrom([]string{"/v1", "/v1/", "/v1///"}).Draw(t, "pathB")

		urlA := fmt.Sprintf("%s://%s.example%s", schemeA, host, pathA)
		urlB := fmt.Sprintf("%s://%s.example%s", schemeB, host, pathB)

		repo := &fakeProfileRepo{profiles: map[string]domainprofile.Profile{}}
		state := &fakeStateRepo{}
		svc := NewProfileService(repo, state, nil, nil, nil)

		if _, err := svc.Add("relay-a", AddProfileInput{
			BaseURL: urlA,
			APIKey:  "sk-a",
		}); err != nil {
			t.Fatalf("first Add() error = %v", err)
		}

		if _, err := svc.Add("relay-b", AddProfileInput{
			BaseURL: urlB,
			APIKey:  "sk-b",
		}); err != nil {
			t.Fatalf("second Add() error = %v", err)
		}
	})
}
