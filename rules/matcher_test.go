package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBasic(t *testing.T) {
	test := "/user/:name"

	km, _ := newRegexKeyMatcher(test)
	match, ok := km.match("/user/john")
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
