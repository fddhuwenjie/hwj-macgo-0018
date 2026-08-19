package domain

var requestTransitions = map[RequestState]map[RequestState]bool{
	RequestStatePending: {
		RequestStateOccupied: true,
		RequestStateFailed:   true,
		RequestStateExpired:  true,
	},
	RequestStateOccupied: {
		RequestStateCommitted: true,
		RequestStateFailed:    true,
		RequestStateTakenOver: true,
		RequestStateExpired:   true,
	},
	RequestStateFailed: {
		RequestStateOccupied: true,
		RequestStateExpired:  true,
	},
	RequestStateTakenOver: {
		RequestStateCommitted: true,
		RequestStateExpired:   true,
		RequestStateOccupied:  true,
	},
	RequestStateCommitted: {},
	RequestStateExpired:   {},
}

var credentialTransitions = map[CredentialState]map[CredentialState]bool{
	CredentialStateIssued: {
		CredentialStateOccupied: true,
		CredentialStateExpired:  true,
	},
	CredentialStateOccupied: {
		CredentialStateCommitted: true,
		CredentialStateFailed:    true,
		CredentialStateTakenOver: true,
		CredentialStateExpired:   true,
	},
	CredentialStateFailed: {
		CredentialStateOccupied: true,
		CredentialStateExpired:  true,
	},
	CredentialStateTakenOver: {
		CredentialStateCommitted: true,
		CredentialStateExpired:   true,
	},
	CredentialStateCommitted: {},
	CredentialStateExpired:   {},
}

func ValidateRequestTransition(from, to RequestState) error {
	return validateTransition(from, to)
}

func ValidateCredentialTransition(from, to CredentialState) error {
	return validateTransition(from, to)
}

func validateTransition(from, to interface{}) error {
	switch f := from.(type) {
	case RequestState:
		t, ok := to.(RequestState)
		if !ok {
			return ErrInvalidTransition
		}
		if f == t {
			return ErrInvalidTransition
		}
		if allowed, ok := requestTransitions[f][t]; ok && allowed {
			return nil
		}
		return ErrInvalidTransition
	case CredentialState:
		t, ok := to.(CredentialState)
		if !ok {
			return ErrInvalidTransition
		}
		if f == t {
			return ErrInvalidTransition
		}
		if allowed, ok := credentialTransitions[f][t]; ok && allowed {
			return nil
		}
		return ErrInvalidTransition
	default:
		return ErrInvalidTransition
	}
}
