package portalinvite

import "testing"

func TestStoredRevokeReason_SpellsEachReasonTheWayTheRowDoes(t *testing.T) {
	for reason, want := range map[RevokeReason]string{
		RevokedEmailChanged: "email_changed",
		RevokedRecordMerged: "record_merged",
		RevokedClientErased: "client_erased",
	} {
		if got := storedRevokeReason(reason); got != want {
			t.Errorf("storedRevokeReason(%q) = %q, want %q", reason, got, want)
		}
	}
}
