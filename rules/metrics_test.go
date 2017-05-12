package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestGetMetricsMetadata(t *testing.T) {
	ctx := context.Background()
	ctx = SetMethod(ctx, "test")
	metadata := GetMetricsMetadata(ctx)
	if assert.NotNil(t, metadata) {
		assert.Equal(t, "test", metadata.Method)
	}
	ctx = context.Background()
	assert.Nil(t, GetMetricsMetadata(ctx))
}
