package usecase

import (
	"errors"
	"testing"

	domainprofile "github.com/kiddingbaby/agx/internal/domain/profile"
)

// TestFinishMutationGuardRollsBackOnPanic verifies that finishMutationGuard
// recovers panics, invokes Rollback (which clears the journal and restores
// state), and re-raises the panic. Earlier the deferred closure checked only
// the named err return and would execute Commit on panic, leaving state half
// applied.
func TestFinishMutationGuardRollsBackOnPanic(t *testing.T) {
	state := &fakeStateRepo{state: domainprofile.State{
		CurrentProfile: "before",
	}}
	svc := &ProfileService{state: state}

	guard := newMutationGuard(svc)
	if err := guard.CaptureState(); err != nil {
		t.Fatalf("CaptureState() error = %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic to propagate")
		}
		if got, _ := r.(string); got != "boom" {
			t.Fatalf("panic value = %v, want %q", r, "boom")
		}
		// Mutation happened before panic; Rollback should have restored the
		// pre-image via state.Save.
		if state.state.CurrentProfile != "before" {
			t.Fatalf("rollback failed to restore CurrentProfile, got %q", state.state.CurrentProfile)
		}
		if guard.active {
			t.Fatalf("guard remained active after rollback")
		}
	}()

	func() {
		var err error
		defer finishMutationGuard(guard, &err)
		state.state.CurrentProfile = "during-mutation"
		_, _ = state.Save(state.state)
		panic("boom")
	}()
}

// TestFinishMutationGuardCommitsOnSuccess sanity-checks that the happy path
// still calls Commit rather than Rollback.
func TestFinishMutationGuardCommitsOnSuccess(t *testing.T) {
	state := &fakeStateRepo{state: domainprofile.State{CurrentProfile: "before"}}
	svc := &ProfileService{state: state}
	guard := newMutationGuard(svc)
	if err := guard.CaptureState(); err != nil {
		t.Fatalf("CaptureState() error = %v", err)
	}

	func() {
		var err error
		defer finishMutationGuard(guard, &err)
		state.state.CurrentProfile = "after"
		_, _ = state.Save(state.state)
	}()

	if guard.active {
		t.Fatalf("guard should be inactive after Commit")
	}
	if state.state.CurrentProfile != "after" {
		t.Fatalf("Commit should keep mutation, got %q", state.state.CurrentProfile)
	}
}

// TestFinishMutationGuardRollsBackOnError verifies the err pointer path runs
// Rollback and preserves the original error.
func TestFinishMutationGuardRollsBackOnError(t *testing.T) {
	state := &fakeStateRepo{state: domainprofile.State{CurrentProfile: "before"}}
	svc := &ProfileService{state: state}
	guard := newMutationGuard(svc)
	if err := guard.CaptureState(); err != nil {
		t.Fatalf("CaptureState() error = %v", err)
	}

	want := errors.New("mutation failed")
	var captured error
	func() {
		var err error
		defer finishMutationGuard(guard, &err)
		state.state.CurrentProfile = "during"
		_, _ = state.Save(state.state)
		err = want
		captured = err
	}()

	if !errors.Is(captured, want) {
		t.Fatalf("err pointer = %v, want %v", captured, want)
	}
	if state.state.CurrentProfile != "before" {
		t.Fatalf("rollback failed, CurrentProfile = %q", state.state.CurrentProfile)
	}
}
