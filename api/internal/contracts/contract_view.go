package contracts

import "doula-cloud/api/internal/staffauth"

// moneyMergeFieldPrefix marks a merge field key as money for ADR-0008's
// scope-vs-money split (#231's "Contract" case) -- e.g. {{money_price}},
// {{money_total_due}}. A naming convention rather than a new schema
// column: a Practice authors its own Contract Template prose, and
// extractMergeFields already treats a key as a bare identifier with no
// other structure, so this is enforceable without a migration. The
// alternative (a tagging column on contract_templates) would need its own
// CRUD surface on PutTemplateHandler for no gain #315's scope needs.
const moneyMergeFieldPrefix = "money_"

// isMoneyMergeFieldKey reports whether key is tagged as money by the
// naming convention above.
func isMoneyMergeFieldKey(key string) bool {
	return len(key) > len(moneyMergeFieldPrefix) && key[:len(moneyMergeFieldPrefix)] == moneyMergeFieldPrefix
}

// ContractView is what GetContractHandler actually encodes -- either a
// ContractScope or a ContractFull. There is no exported function that
// returns the raw MergeFieldValues map with money still in it: ReadContract
// is the only way out of this package for a read, and it always returns
// one of these two types, chosen by the Reader it was given -- "no price
// key reachable, not just a redacted value" for anyone but Owner/Admin.
type ContractView interface {
	isContractView()
}

// ContractScope is what every role that can reach the Contract at all
// gets: Visit counts, dates, on-call terms -- whatever merge fields
// aren't money. MergeFields keeps every key parsed from the prose
// (including money ones, whose names alone don't disclose a value and
// are already visible on the open Contract Template); Values omits any
// money key entirely.
type ContractScope struct {
	EngagementID string           `json:"engagementId"`
	Status       string           `json:"status"`
	Prose        string           `json:"prose"`
	MergeFields  []string         `json:"mergeFields"`
	Values       MergeFieldValues `json:"values"`
}

// ContractFull is ContractScope plus the money field values, for a
// Reader that proves Owner or Admin -- never an employee or contractor
// Doula (ADR-0008: "her own agreed fee only ... never the Practice's
// price").
type ContractFull struct {
	ContractScope
	MoneyValues MergeFieldValues `json:"moneyValues"`
}

// coverage:ignore reason: marker methods that seal ContractView -- never invoked, only satisfied
func (ContractScope) isContractView() {}

// coverage:ignore reason: marker methods that seal ContractView -- never invoked, only satisfied
func (ContractFull) isContractView() {}

// ReadContract shapes full to the view r's roles are entitled to.
func ReadContract(r staffauth.Reader, full ContractResponse) ContractView {
	scopeValues := MergeFieldValues{}
	moneyValues := MergeFieldValues{}
	for _, key := range full.MergeFields {
		if isMoneyMergeFieldKey(key) {
			moneyValues[key] = full.Values[key]
		} else {
			scopeValues[key] = full.Values[key]
		}
	}
	scope := ContractScope{
		EngagementID: full.EngagementID,
		Status:       full.Status,
		Prose:        full.Prose,
		MergeFields:  full.MergeFields,
		Values:       scopeValues,
	}
	if r.Has("owner") || r.Has("admin") {
		return ContractFull{ContractScope: scope, MoneyValues: moneyValues}
	}
	return scope
}
