package jitter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDuration(t *testing.T) {
	for _, tc := range []struct {
		description      string
		duration         Duration
		expectLowerBound time.Duration
		expectUpperBound time.Duration
	}{
		{
			description:      "happy path",
			duration:         NewDuration(5*time.Minute, 0.2 /* 20% */),
			expectLowerBound: 4 * time.Minute,
			expectUpperBound: 6 * time.Minute,
		},
		{
			description:      "zero value",
			duration:         Duration{},
			expectLowerBound: 0,
			expectUpperBound: 0,
		},
		{
			description:      "percent too high",
			duration:         NewDuration(5*time.Minute, 100),
			expectLowerBound: 0,
			expectUpperBound: 10 * time.Minute,
		},
		{
			description:      "percent too low",
			duration:         NewDuration(5*time.Minute, -100),
			expectLowerBound: 5 * time.Minute,
			expectUpperBound: 5 * time.Minute,
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			const maxAttempts = 500
			for i := 0; i < maxAttempts; i++ {
				d := tc.duration.Next()
				assert.LessOrEqual(t, tc.expectLowerBound, d)
				assert.GreaterOrEqual(t, tc.expectUpperBound, d)
			}
		})
	}
}
