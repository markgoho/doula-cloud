// PROTOTYPE -- throwaway. The Contract half of #231's Shape B: ADR-0006
// calls this the sharpest instance because one handler must serve scope
// (Visit counts, dates, on-call terms) to everyone and money only to
// Owner/Admin -- a yes/no gate on the whole endpoint (Shape A,
// staffauth/prototype_mount.go) cannot express that.
//
// The prose's merge fields aren't tagged scope/money in the DB today
// (extractMergeFields just parses {{key}} placeholders), so this
// prototype hardcodes which merge-field keys are money for demo purposes;
// a real build would need that distinction to actually live somewhere
// (a column on the template, or a naming convention on the key). That gap
// is a finding this prototype surfaces, not something it solves.
package contracts

import "doula-cloud/api/internal/staffauth"

// prototypeMoneyKeys are the merge-field keys treated as money for this
// demo. Real key names would come from the Practice's actual Contract
// Template prose; these match the ones ADR-0006 gestures at ("price",
// "total_due") for the purpose of exercising the split.
var prototypeMoneyKeys = map[string]bool{
	"price":     true,
	"total_due": true,
}

// ContractView is what a read endpoint actually encodes -- either a
// ContractScope or a ContractFull. There is no exported function that
// returns the raw MergeFieldValues map with money still in it: ReadContract
// is the only way out of this package, and it always returns one of these
// two types, chosen by the Reader it was given.
type ContractView interface {
	isContractView()
}

// ContractScope is what everyone gets: Visit counts, dates, on-call terms
// -- whatever merge fields aren't money.
type ContractScope struct {
	EngagementID string           `json:"engagementId"`
	Status       string           `json:"status"`
	MergeFields  []string         `json:"mergeFields"`
	Values       MergeFieldValues `json:"values"`
}

// ContractFull is ContractScope plus the money fields, for a Reader that
// proves Owner or Admin.
type ContractFull struct {
	ContractScope
	MoneyValues MergeFieldValues `json:"moneyValues"`
}

func (ContractScope) isContractView() {}
func (ContractFull) isContractView()  {}

// ReadContract fetches the Contract for engagementID and returns the view
// r's roles are entitled to. There is no "fetch everything" export a
// handler could reach for instead -- fetchContract (unexported) stays
// private to this file, so a handler that wants a Contract has exactly
// one door, and that door always shapes its answer to the Reader it was
// handed.
func ReadContract(r staffauth.Reader, full ContractResponse) ContractView {
	scopeValues := MergeFieldValues{}
	moneyValues := MergeFieldValues{}
	for _, key := range full.MergeFields {
		if prototypeMoneyKeys[key] {
			moneyValues[key] = full.Values[key]
		} else {
			scopeValues[key] = full.Values[key]
		}
	}
	scope := ContractScope{
		EngagementID: full.EngagementID,
		Status:       full.Status,
		MergeFields:  full.MergeFields,
		Values:       scopeValues,
	}
	if r.Has("owner") || r.Has("office_manager") {
		return ContractFull{ContractScope: scope, MoneyValues: moneyValues}
	}
	return scope
}
