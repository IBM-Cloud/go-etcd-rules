package jitter

import (
	"math/rand"
	"time"
)

// Duration is a time.Duration with variance. Successive calls to Next() return new randomized durations.
// The zero-value returns 0 for every time duration.
type Duration struct {
	base          time.Duration
	jitterPercent float64
}

// NewDuration returns a new Duration with a base time duration and a percentage of jitter
func NewDuration(base time.Duration, jitterPercent float64) Duration {
	return Duration{
		base:          base,
		jitterPercent: clampPercent(jitterPercent),
	}
}

// Next returns a new, randomized duration from the interval base±jitterPercent
func (d Duration) Next() time.Duration {
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
		dNano           = float64(d.base.Nanoseconds())
		random          = rand.Float64() // in range [0, 1)
		randomPlusMinus = 2*random - 1   // in range [-0.5, 0.5)
		resultNano      = dNano + dNano*d.jitterPercent*randomPlusMinus
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
