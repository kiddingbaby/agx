package agent

import "testing"

func TestDefaultAgentsReturnsCopy(t *testing.T) {
	first := DefaultAgents()
	second := DefaultAgents()

	if len(first) == 0 || len(second) == 0 {
		t.Fatal("DefaultAgents() returned empty list")
	}

	first[0].Name = "mutated"
	if second[0].Name == "mutated" {
		t.Fatal("DefaultAgents() should return a copy")
	}
}

func TestFind(t *testing.T) {
	if _, ok := Find("claude-code"); !ok {
		t.Fatal("Find canonical name should work")
	}

	agent, ok := Find("claude")
	if !ok {
		t.Fatal("Find alias should work")
	}
	if agent.Name != "claude-code" {
		t.Fatalf("alias resolve result = %s, want claude-code", agent.Name)
	}

	if _, ok := Find("unknown-agent"); ok {
		t.Fatal("Find unknown should fail")
	}
}

func TestAliasMapReturnsCopy(t *testing.T) {
	m1 := AliasMap()
	m2 := AliasMap()

	m1["claude"] = "changed"
	if m2["claude"] == "changed" {
		t.Fatal("AliasMap() should return a copy")
	}
}
