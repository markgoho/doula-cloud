package apierr_test

import "testing"

// TestDeliberateCIFailureForIssue1186 exists only to verify, on this PR
// branch, that the api job's summary still names a failing package and
// test after #1186's omit change -- without opening the raw log. Removed
// before merge; see the issue for the observed result.
func TestDeliberateCIFailureForIssue1186(t *testing.T) {
	t.Fatal("deliberate failure for #1186 CI summary verification")
}
