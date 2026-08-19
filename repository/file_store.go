package repository

import "context"

// SaveThenCheckContext models a persistence wait that observes cancellation
// only after the durable write has completed.
func SaveThenCheckContext(ctx context.Context, save func() error) error {
	if save == nil {
		return ErrInvalid
	}
	if err := save(); err != nil {
		return err
	}
	return ctx.Err()
}
