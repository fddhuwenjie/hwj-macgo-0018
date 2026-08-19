package clockidcodec

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

type IDGenerator struct {
	clock Clock
}

func NewIDGenerator(c Clock) *IDGenerator {
	if c == nil {
		c = RealClock{}
	}
	return &IDGenerator{clock: c}
}

func (g *IDGenerator) NewID(prefix string) (string, error) {
	if err := ValidatePrefix(prefix); err != nil {
		return "", err
	}
	now := g.clock.Now()
	ts := now.UnixNano()
	randomBytes := make([]byte, 12)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	randomPart := base64.RawURLEncoding.EncodeToString(randomBytes)
	if prefix == "" {
		return fmt.Sprintf("%d_%s", ts, randomPart), nil
	}
	return fmt.Sprintf("%s_%d_%s", prefix, ts, randomPart), nil
}
