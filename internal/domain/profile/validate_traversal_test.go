package profile

import "testing"

// TestValidateProfileNameRejectsTraversal locks down rejection of names that
// resolve to filesystem traversal tokens. Previously names like ".", "..",
// "..." passed validation and, once interpolated into managed-context paths,
// caused filepath.Join to escape the per-target directory.
func TestValidateProfileNameRejectsTraversal(t *testing.T) {
	bad := []string{".", "..", "...", "....", "/", "\\", "a/b", "a\\b"}
	for _, name := range bad {
		if err := ValidateProfileName(name); err == nil {
			t.Errorf("ValidateProfileName(%q) = nil, want error", name)
		}
	}
}

// TestValidateTargetNameRejectsTraversal applies the same rule to target
// names.
func TestValidateTargetNameRejectsTraversal(t *testing.T) {
	bad := []string{".", "..", "...", "....", "/", "\\", "a/b", "..."}
	for _, name := range bad {
		if err := ValidateTargetName(name); err == nil {
			t.Errorf("ValidateTargetName(%q) = nil, want error", name)
		}
	}
}

// TestValidateNameAcceptsRealistic keeps acceptable names working.
func TestValidateNameAcceptsRealistic(t *testing.T) {
	good := []string{"work", "relay-prod", "v1.2", "a", "a.b", "a_b", "1.0", "us-east-1"}
	for _, name := range good {
		if err := ValidateProfileName(name); err != nil {
			t.Errorf("ValidateProfileName(%q) error = %v", name, err)
		}
		if err := ValidateTargetName(name); err != nil {
			t.Errorf("ValidateTargetName(%q) error = %v", name, err)
		}
	}
}
