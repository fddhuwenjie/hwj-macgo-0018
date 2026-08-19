package scheduler

import (
	"errors"
	"math"
	"time"
)

var ErrRetryExhausted = errors.New("scheduler: retry attempts exhausted")

type RetryPolicy interface {
	MaxAttempts() int
	NextDelay(attempt int) time.Duration
}

type ExponentialBackoff struct {
	Base         time.Duration
	Factor       float64
	Max          time.Duration
	AttemptLimit int
}

func NewExponentialBackoff(base time.Duration, factor float64, max time.Duration, attemptLimit int) *ExponentialBackoff {
	if base <= 0 {
		base = time.Second
	}
	if factor <= 1 {
		factor = 2
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	if attemptLimit <= 0 {
		attemptLimit = 3
	}
	return &ExponentialBackoff{
		Base:         base,
		Factor:       factor,
		Max:          max,
		AttemptLimit: attemptLimit,
	}
}

func (b *ExponentialBackoff) MaxAttempts() int {
	return b.AttemptLimit
}

func (b *ExponentialBackoff) NextDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	base := float64(b.Base)
	factor := math.Pow(b.Factor, float64(attempt-1))
	delay := time.Duration(base * factor)
	if delay > b.Max {
		return b.Max
	}
	return delay
}

type ConstantBackoff struct {
	Delay        time.Duration
	AttemptLimit int
}

func (c *ConstantBackoff) MaxAttempts() int {
	return c.AttemptLimit
}

func (c *ConstantBackoff) NextDelay(attempt int) time.Duration {
	if c.Delay <= 0 {
		return time.Second
	}
	return c.Delay
}
