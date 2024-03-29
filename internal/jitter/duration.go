package jitter

import (
	"math/rand" // nolint:gosec // Only generating random durations, which is not security-sensitive. A pseudo-random number generator is ok.
	"time"

	"go.uber.org/zap/zapcore"
)

// DurationGenerator generates time.Durations with variance called "jitter".
// Successive calls to Generate() return new randomized durations.
//
// The zero-value returns 0 for every duration.
type DurationGenerator struct {
	base          time.Duration
	jitterPercent float64
}

// MarshalLogObject allow zap logging
func (g *DurationGenerator) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddDuration("base", g.base)
	enc.AddFloat64("jitterPercent", g.jitterPercent)
	return nil
}

// NewDurationGenerator returns a new DurationGenerator with a base time duration and a percentage of jitter
func NewDurationGenerator(base time.Duration, jitterPercent float64) DurationGenerator {
	return DurationGenerator{
		base:          base,
		jitterPercent: clampPercent(jitterPercent),
	}
}

// Generate returns a new, randomized duration from the interval base±jitterPercent
func (g DurationGenerator) Generate() time.Duration {
	/*
		Example formula:
		base = 100ns
		jitterPercent = 0.1 (10%)
		Result should be within 10% of 100ns. Or: 100ns±10%

		RANDOM = 0.3
		100 + 100 * 0.1 * (2*0.3 - 1) = 96ns
		RANDOM = 0.9
		100 + 100 * 0.1 * (2*0.9 - 1) = 108ns
	*/
	var (
		dNano           = float64(g.base.Nanoseconds())
		random          = rand.Float64() // #nosec G404 -- Generating random durations is not security-sensitive. A pseudo-random number generator is ok. in range [0, 1)
		randomPlusMinus = 2*random - 1   // in range [-0.5, 0.5)
		resultNano      = dNano + dNano*g.jitterPercent*randomPlusMinus
	)
	return time.Duration(resultNano) * time.Nanosecond
}

func clampPercent(f float64) float64 {
	switch {
	case f < 0:
		return 0
	case f > 1:
		return 1
	default:
		return f
	}
}
