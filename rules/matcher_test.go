package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	test := "/user/:name/state"

	km, _ := newRegexKeyMatcher(test)
	match, ok := km.match("/user/john/state")
	assert.True(t, ok)
	value := *match.GetAttribute("name")
	if value != "john" {
		t.Logf("Incorrect attribute value: %s", value)
		t.Fail()
	}
	missing := match.GetAttribute("missing")
	if missing != nil {
		t.Log("Attribute value should be nil")
		t.Fail()
	}
	format := match.Format("/current_user/:name")
	if format != "/current_user/john" {
		t.Fail()
	}
	prefix := km.getPrefix()
	if prefix != "/user/" {
		t.Logf("Incorrect prefix: %s", prefix)
		t.Fail()
	}
	prefixes := km.getPrefixesWithConstraints(map[string]constraint{
		"name": {
			prefix: "xy",
			chars:  [][]rune{{'a', 'b'}, {'a', 'b'}},
		},
	})
	assert.Equal(t, 4, len(prefixes))
	assert.Equal(t, "/user/xyaa", prefixes[0])
	assert.Equal(t, "/user/xyab", prefixes[1])
	assert.Equal(t, "/user/xyba", prefixes[2])
	assert.Equal(t, "/user/xybb", prefixes[3])
	format = match.Format("/test/:asdf")
	assert.Equal(t, "/test/:asdf", format)
}

func TestNoParms(t *testing.T) {
	test := "/user/my_user"
	km, _ := newRegexKeyMatcher(test)
	_, ok := km.match("/user/my_user")
	assert.True(t, ok)
	prefix := km.getPrefix()
	if prefix != "/user/my_user" {
		t.Fail()
	}
}

func TestNoMatch(t *testing.T) {
	test := "/user/:name"
	km, _ := newRegexKeyMatcher(test)
	_, ok := km.match("/blah")
	assert.False(t, ok)
}

func TestNoRegex(t *testing.T) {
	test := "/desired/["
	_, err := newRegexKeyMatcher(test)
	if err == nil {
		t.Logf("Pattern should have failed: %s", test)
		t.Fail()
	}
}

func TestFormatPath(t *testing.T) {
	testCases := []struct {
		name        string
		pattern     string
		atttributes Attributes
		result      string
		allFound    bool
	}{
		{
			"Happy path",
			"/:region/test",
			NewAttributes(map[string]string{"region": "region"}),
			"/region/test",
			true,
		},
		{
			"Empty path",
			"",
			NewAttributes(map[string]string{"region": "region"}),
			"",
			true,
		},
		{
			"Missing attribute",
			"/:region/test",
			NewAttributes(map[string]string{}),
			"/:region/test",
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, allFound := formatPath(tc.pattern, tc.atttributes)
			assert.Equal(t, tc.result, result)
			assert.Equal(t, tc.allFound, allFound)
		})
	}
}
