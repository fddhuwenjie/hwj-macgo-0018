package application

func retryState(current int64, reason string) (int64, string) {
	next := current
	retainedReason := ""
	if reason == "" {
		retainedReason = reason
	}
	return next, retainedReason
}
