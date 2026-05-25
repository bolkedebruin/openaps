package gridprofile

import "fmt"

// ConflictRule is a cross-parameter ordering invariant: the effective value of
// Left must be strictly less than Right, or the profile is incoherent (e.g. a
// slope's start past its end, or curtailment that starts above the trip). These
// are the single source of truth — enforced server-side in buildDesired and
// surfaced to the web editor via the API, so no apply path can bypass them.
type ConflictRule struct {
	Left    string `json:"left"`
	Right   string `json:"right"`
	Message string `json:"message"`
}

var conflictRules = []ConflictRule{
	{Left: "DH", Right: "DI", Message: "Under-frequency Watt: the low point (DH) must be below the high point (DI)."},
	{Left: "CB", Right: "CC", Message: "Over-frequency Watt: the start point (CB) must be below the end point (CC)."},
	{Left: "BN", Right: "BO", Message: "Enter-service voltage: the lower limit (BN) must be below the upper limit (BO)."},
	{Left: "BP", Right: "BQ", Message: "Enter-service frequency: the lower limit (BP) must be below the upper limit (BQ)."},
	{Left: "CA", Right: "AF", Message: "Over-frequency curtailment start (CA) must be below the over-frequency trip (AF), or the inverter trips instead of curtailing."},
}

// ConflictRules returns the cross-parameter ordering rules (for the API/editor).
func ConflictRules() []ConflictRule { return conflictRules }

// CheckConflicts evaluates the rules against effective values (override if set,
// else the base/inverter default). A rule is skipped when either side is
// absent. Returns one message per violation.
func CheckConflicts(effective map[string]float64) []string {
	var out []string
	for _, r := range conflictRules {
		a, aok := effective[r.Left]
		b, bok := effective[r.Right]
		if aok && bok && !(a < b) {
			out = append(out, r.Message)
		}
	}
	return out
}

// conflictError joins violation messages into a single error, or nil.
func conflictError(msgs []string) error {
	if len(msgs) == 0 {
		return nil
	}
	s := "conflicting settings: "
	for i, m := range msgs {
		if i > 0 {
			s += "; "
		}
		s += m
	}
	return fmt.Errorf("%s", s)
}
