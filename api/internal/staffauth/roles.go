package staffauth

import (
	"slices"
)

// validRoles is the practice_role enum from 00002_practice_staff_tenancy.sql,
// mirrored here so role-assignment requests can be validated before they
// ever reach Postgres.
var validRoles = map[string]bool{roleOwner: true, roleAdmin: true, "doula": true}

// The 403 sentences a role refusal writes, named once so the in-handler
// RequireOwner/RequireOwnerOrAdmin and the mount-level requireAnyRole can
// never drift onto two wordings for the same refusal (#1031). Each is the
// sentence the screen shows as it is (#1884), so it speaks to the reader
// and names who can, with the role lowercase as the Voice section's rule 2
// writes a person not yet known (#2261).
const (
	msgOwnerOnlyRefused    = "You need to be an owner of this Practice to do that."
	msgOwnerOrAdminRefused = "You need to be an owner or admin of this Practice to do that."
	// msgRoleRefusedFallback is what refusalMessage says for a role list
	// it has no seat-naming sentence for. It names no seat, so it stays
	// true for any list, including one a future route declares -- a
	// wrong seat in the sentence would be worse than none.
	msgRoleRefusedFallback = "Your role at this Practice does not let you do that."
)

// refusalMessage is the 403 sentence for a caller who holds none of roles.
// OwnerOnly and OwnerAndAdmin -- the only two declarations the mount-gated
// routes use -- get the sentence naming the seat, matching what
// RequireOwner and RequireOwnerOrAdmin write; anything else gets
// msgRoleRefusedFallback. Order in roles does not matter.
func refusalMessage(roles []string) string {
	hasOwner, hasAdmin := slices.Contains(roles, roleOwner), slices.Contains(roles, roleAdmin)
	switch {
	case len(roles) == 1 && hasOwner:
		return msgOwnerOnlyRefused
	case len(roles) == 2 && hasOwner && hasAdmin:
		return msgOwnerOrAdminRefused
	default:
		return msgRoleRefusedFallback
	}
}

// MsgContractorMoneyRefused is the 403 body every route ADR-0008's money
// row (as amended by #282) refuses a contractor Doula with -- named once
// so NotAmbientContractor and any route that checks the Reader itself can
// never drift onto two different wordings for the same refusal.
const MsgContractorMoneyRefused = "You contract for this Practice, so you cannot see its money. You can see your own agreed fee on work you are attached to."
