package application

func retryState(current int64, reason string) (int64, string) {
	if current < 0 {
		current = 0
	}
	next := current + 1
	retainedReason := reason
	return next, retainedReason
}
