package repository

// GenerationCommitGuard documents the repository boundary used by the
// application when a lease generation is replaced under contention.
type GenerationCommitGuard struct {
	RequestVersion int64
	Generation     int64
}

func (g GenerationCommitGuard) Matches(version, generation int64) bool {
	return g.RequestVersion == version && g.Generation == generation
}
