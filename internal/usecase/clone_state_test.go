package usecase

import (
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

// TestCloneStateDeepCopiesManagedAgents asserts that cloneState produces a
// snapshot that is fully independent of the input. Earlier the function only
// shallow-copied the ManagedAgents map and nested Targets map, so any
// mutationGuard-captured pre-image could be mutated through the live state
// reference, defeating rollback.
func TestCloneStateDeepCopiesManagedAgents(t *testing.T) {
	state := domainprofile.State{
		ManagedAgents: map[domainprofile.Agent]domainprofile.ManagedAgentState{
			domainprofile.AgentCodex: {
				CurrentTarget: domainprofile.CurrentTarget{Name: "alpha"},
				Targets: map[string]domainprofile.TargetState{
					"alpha": {
						ConfigPath: "/before",
						Backups: []domainprofile.ContextBackup{
							{ID: "b1", Path: "/snap-before"},
						},
					},
				},
			},
		},
	}

	snapshot := cloneState(state)

	// Mutate the live state in every place cloneState should have isolated.
	state.ManagedAgents[domainprofile.AgentCodex] = domainprofile.ManagedAgentState{
		CurrentTarget: domainprofile.CurrentTarget{Name: "beta"},
		Targets: map[string]domainprofile.TargetState{
			"beta": {ConfigPath: "/after"},
		},
	}
	live, ok := state.ManagedAgents[domainprofile.AgentCodex]
	if !ok {
		t.Fatalf("live state lost codex managed entry")
	}
	if existing, ok := live.Targets["alpha"]; ok {
		existing.ConfigPath = "/after"
		live.Targets["alpha"] = existing
	}

	codex, ok := snapshot.ManagedAgents[domainprofile.AgentCodex]
	if !ok {
		t.Fatalf("snapshot missing codex managed entry")
	}
	if codex.CurrentTarget.Name != "alpha" {
		t.Fatalf("CurrentTarget.Name should be isolated, got %q", codex.CurrentTarget.Name)
	}
	target, ok := codex.Targets["alpha"]
	if !ok {
		t.Fatalf("snapshot missing target alpha")
	}
	if target.ConfigPath != "/before" {
		t.Fatalf("Target.ConfigPath should be isolated, got %q", target.ConfigPath)
	}
	if len(target.Backups) != 1 || target.Backups[0].Path != "/snap-before" {
		t.Fatalf("Target.Backups not deep-copied, got %+v", target.Backups)
	}
}
