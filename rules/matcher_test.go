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

func formatPathTest(pattern string, m Attributes) (string, bool) {
	sb := new(strings.Builder)
	// If the formatted string can fit into 2x the length of the pattern
	// (and mapAttributes is the attribute implementation used)
	// this will be the only allocation
	sb.Grow(len(pattern) * 2)

	all := true
	var segment string
	for found := true; found; {
		segment, pattern, found = strings.Cut(pattern, "/")
		switch {
		case segment == "":
		case strings.HasPrefix(segment, ":"):
			sb.WriteByte('/')
			// The Attributes interface is rather inefficient
			// It requires an allocation/derefence to look up
			// strings from a map because it replaces the built-in
			// (string, bool) return with a *string return.
			// To save allocations we'll avoid calling GetAttribute
			// if we can directly access a map
			if ma, ok := m.(*mapAttributes); ok {
				if attr, ok := ma.values[segment[1:]]; ok {
					sb.WriteString(attr)
				} else {
					all = false
					sb.WriteString(segment)
				}
			} else {
				attr := m.GetAttribute(segment[1:])
				if attr == nil {
					sb.WriteString(segment)
					all = false
				} else {
					sb.WriteString(*attr)
				}
			}
		default:
			sb.WriteByte('/')
			sb.WriteString(segment)
		}
	}
	return sb.String(), all
}

func BenchmarkFormatPath(b *testing.B) {
	cases := []struct {
		name    string
		attr    Attributes
		pattern string
	}{
		{
			name:    "3 matching",
			attr:    NewAttributes(map[string]string{"first": "one", "second": "two", "third": "three"}),
			pattern: "/first/:first/second/:second/third/:third",
		},
		{
			name:    "5 matching",
			attr:    NewAttributes(map[string]string{"a": "aaaaaaaaaa", "b": "aaaaaaaaaa", "c": "aaaaaaaaaa", "d": "aaaaaaaaaa", "e": "eeeeeeeeee"}),
			pattern: "first/:a/second/:b/third/:c/fourth/:d/fifth/:e/sixth",
		},
		{
			name:    "10 matching",
			attr:    NewAttributes(map[string]string{"a": "aaaaaaaaaa", "b": "aaaaaaaaaa", "c": "aaaaaaaaaa", "d": "aaaaaaaaaa", "e": "eeeeeeeeee"}),
			pattern: "first/:a/second/:b/third/:c/fourth/:d/fifth/:e/first/:a/second/:b/third/:c/fourth/:d/fifth/:e/sixth",
		},
		{
			name:    "3 no matches",
			attr:    NewAttributes(map[string]string{"x": "one", "y": "two", "z": "three"}),
			pattern: "/first/:first/second/:second/third/:third",
		},
		{
			name:    "5 no matches",
			attr:    NewAttributes(map[string]string{"1": "aaaaaaaaaa", "2": "aaaaaaaaaa", "3": "aaaaaaaaaa", "4": "aaaaaaaaaa", "5": "eeeeeeeeee"}),
			pattern: "first/:a/second/:b/third/:c/fourth/:d/fifth/:e/sixth",
		},
		{
			name:    "10 no matches",
			attr:    NewAttributes(map[string]string{"1": "aaaaaaaaaa", "2": "aaaaaaaaaa", "3": "aaaaaaaaaa", "4": "aaaaaaaaaa", "5": "eeeeeeeeee"}),
			pattern: "first/:a/second/:b/third/:c/fourth/:d/fifth/:e/first/:a/second/:b/third/:c/fourth/:d/fifth/:e/sixth",
		},
	}

	for _, tc := range cases {
		b.Run("old:"+tc.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				formatPath(tc.pattern, tc.attr)
			}
		})
		b.Run("new:"+tc.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				formatPathTest(tc.pattern, tc.attr)
			}
		})
	}
}

func BenchmarkFormatPathProfile(b *testing.B) {
	attr := NewAttributes(map[string]string{"a": "aaaaaaaaaa", "b": "aaaaaaaaaa", "c": "aaaaaaaaaa", "d": "aaaaaaaaaa", "e": "eeeeeeeeee"})
	pattern := "first/:a/second/:b/third/:c/fourth/:d/fifth/:e/first/:a/second/:b/third/:c/fourth/:d/fifth/:e/sixth"

	for n := 0; n < b.N; n++ {
		formatPathTest(pattern, attr)
	}
}

func TestFormatPath(t *testing.T) {
	cases := []struct {
		name       string
		attr       Attributes
		pattern    string
		expectstr  string
		expectbool bool
	}{
		{
			name:       "3 matching",
			attr:       NewAttributes(map[string]string{"first": "one", "second": "two", "third": "three"}),
			pattern:    "/first/:first/second/:second/third/:third",
			expectstr:  "/first/one/second/two/third/three",
			expectbool: true,
		},
		{
			name:       "5 matching",
			attr:       NewAttributes(map[string]string{"a": "aaaaaaaaaa", "b": "aaaaaaaaaa", "c": "aaaaaaaaaa", "d": "aaaaaaaaaa", "e": "eeeeeeeeee"}),
			pattern:    "first/:a/second/:b/third/:c/fourth/:d/fifth/:e/sixth",
			expectstr:  "/first/aaaaaaaaaa/second/aaaaaaaaaa/third/aaaaaaaaaa/fourth/aaaaaaaaaa/fifth/eeeeeeeeee/sixth",
			expectbool: true,
		},
		{
			name:       "empty segment",
			attr:       NewAttributes(map[string]string{}),
			pattern:    "a///b",
			expectstr:  "/a/b",
			expectbool: true,
		},
		{
			name:       "empty",
			attr:       NewAttributes(map[string]string{}),
			pattern:    "",
			expectstr:  "",
			expectbool: true,
		},
		{
			name:       "single slash",
			attr:       NewAttributes(map[string]string{}),
			pattern:    "/",
			expectstr:  "",
			expectbool: true,
		},
		{
			name:       "empty paths",
			attr:       NewAttributes(map[string]string{}),
			pattern:    "///",
			expectstr:  "",
			expectbool: true,
		},
		{
			name:       "empty pattern",
			attr:       NewAttributes(map[string]string{}),
			pattern:    "/:",
			expectstr:  "/:",
			expectbool: false,
		},
		{
			name:       "empty pattern",
			attr:       NewAttributes(map[string]string{"": "test"}),
			pattern:    "/:",
			expectstr:  "/test",
			expectbool: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actualstr, actualbool := formatPath(tc.pattern, tc.attr)
			require.Equal(t, tc.expectstr, actualstr)
			require.Equal(t, tc.expectbool, actualbool)
		})
		t.Run("new:"+tc.name, func(t *testing.T) {
			actualstr, actualbool := formatPathTest(tc.pattern, tc.attr)
			require.Equal(t, tc.expectstr, actualstr)
			require.Equal(t, tc.expectbool, actualbool)
		})
	}
}
