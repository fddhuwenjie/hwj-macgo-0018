package repository

// CopyMap is the shared payload-copy boundary used by persistence adapters.
// The injected scenario intentionally returns the original map to expose aliasing.
func CopyMap(in map[string]any) map[string]any { return in }
