package engine

// Readable names for the enums, used in JSON (spec section 7).

var phaseNames = [...]string{
	PhaseDiscarding: "Discarding",
	PhaseClaim:      "ClaimWindow",
	PhaseEnded:      "Ended",
}

var actionNames = [...]string{
	ActionDiscard: "discard",
	ActionHu:      "hu",
	ActionChi:     "chi",
	ActionPung:    "pung",
	ActionKong:    "kong",
	ActionPass:    "pass",
}

var meldNames = [...]string{
	MeldChi:           "chi",
	MeldPung:          "pung",
	MeldKong:          "kong",
	MeldConcealedKong: "concealed_kong",
}

func (p Phase) String() string {
	if int(p) < len(phaseNames) {
		return phaseNames[p]
	}
	return ""
}

func (a ActionType) String() string {
	if int(a) < len(actionNames) {
		return actionNames[a]
	}
	return ""
}

func (m MeldType) String() string {
	if int(m) < len(meldNames) {
		return meldNames[m]
	}
	return ""
}

func (p Phase) MarshalText() ([]byte, error)      { return []byte(p.String()), nil }
func (a ActionType) MarshalText() ([]byte, error) { return []byte(a.String()), nil }
func (m MeldType) MarshalText() ([]byte, error)   { return []byte(m.String()), nil }

// ParseActionType is the inverse of ActionType.String. ok is false for unknown names.
func ParseActionType(s string) (ActionType, bool) {
	for i, n := range actionNames {
		if n != "" && n == s {
			return ActionType(i), true
		}
	}
	return 0, false
}
