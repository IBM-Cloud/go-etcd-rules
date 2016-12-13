package rules

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestWatcher(t *testing.T) {
	value1 := "value1"
	tkw := testKeyWatcher{
		keys:   []string{"key1", "key2"},
		values: []*string{&value1, nil},
		errors: []error{nil, errors.New("Error")},
	}
	logger := getTestLogger()
	kp := testKeyProcessor{}
	w := watcher{
		api:    newMapReadAPI(),
		kw:     &tkw,
		kp:     &kp,
		logger: logger,
	}
	w.singleRun()
	assert.Equal(t, "key1", kp.keys[0])
	assert.Equal(t, &value1, kp.values[0])
	assert.Equal(t, context.Background(), w.getContext())
	w.singleRun()
}

type testKeyWatcher struct {
	keys   []string
	values []*string
	errors []error
	index  int
}

func (tkw *testKeyWatcher) next() (string, *string, error) {
	index := tkw.index
	tkw.index = index + 1
	return tkw.keys[index], tkw.values[index], tkw.errors[index]
}
