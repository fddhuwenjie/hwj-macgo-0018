package clockidcodec

import "time"

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

type FrozenClock struct {
	T time.Time
}

func NewFrozenClock(t time.Time) *FrozenClock {
	return &FrozenClock{T: t.UTC()}
}

func (f *FrozenClock) Now() time.Time {
	if f == nil {
		return time.Time{}
	}
	return f.T
}
