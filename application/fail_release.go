package application

// retryState advances the execution generation for a retry and carries the
// previous failure reason forward, so a retried execution enters a new
// generation without discarding the original failure record.
func retryState(current int64, reason string) (int64, string) {
	return current + 1, reason
}
