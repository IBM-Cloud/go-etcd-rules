package rules

import (
	"regexp"
	"strings"
)

type keyMatcher interface {
	getPrefix() string
	match(string) (keyMatch, bool)
	getPattern() string
}

type regexKeyMatcher struct {
	regex    *regexp.Regexp
	fieldMap map[string]int
	pattern  string
}

type keyMatch interface {
	GetAttribute(name string) *string
	Format(pattern string) string
}

func (rkm *regexKeyMatcher) getPrefix() string {
	end := strings.Index(rkm.pattern, ":")
	if end == -1 {
		end = len(rkm.pattern)
	}
	return rkm.pattern[0:end]
}

func (rkm *regexKeyMatcher) getPattern() string {
	return rkm.pattern
}

type regexKeyMatch struct {
	matchStrings []string
	fieldMap     map[string]int
}

func newKeyMatch(path string, kmr *regexKeyMatcher) *regexKeyMatch {
	results := kmr.regex.FindStringSubmatch(path)
	if results == nil {
		return nil
	}
	km := &regexKeyMatch{
		matchStrings: results,
		fieldMap:     kmr.fieldMap,
	}
	return km
}

func (m *regexKeyMatch) GetAttribute(name string) *string {
	index, ok := m.fieldMap[name]
	if !ok {
		return nil
	}
	result := m.matchStrings[index]
	return &result
}

func (m *regexKeyMatch) Format(pattern string) string {
	return formatWithAttributes(pattern, m)
}
func formatWithAttributes(pattern string, m Attributes) string {
	paths := strings.Split(pattern, "/")
	result := ""
	for _, path := range paths {
		if len(path) == 0 {
			continue
		}
		result = result + "/"
		if strings.HasPrefix(path, ":") {
			attr := m.GetAttribute(path[1:])
			if attr == nil {
				s := "XXX"
				attr = &s
			}
			result = result + *attr
		} else {
			result = result + path
		}
	}
	return result
}

// Keep the bool return value, because it's tricky to check for null
// references when dealing with interfaces
func (rkm *regexKeyMatcher) match(path string) (keyMatch, bool) {
	m := newKeyMatch(path, rkm)
	if m == nil {
		return nil, false
	}
	return m, true
}

func newRegexKeyMatcher(pattern string) (*regexKeyMatcher, error) {
	fields, regexString := parsePath(pattern)
	regex, err := regexp.Compile(regexString)
	if err != nil {
		return nil, err
	}
	return &regexKeyMatcher{
		regex:    regex,
		fieldMap: fields,
		pattern:  pattern,
	}, nil
}

func parsePath(pattern string) (map[string]int, string) {
	paths := strings.Split(pattern, "/")
	regex := ""
	fields := make(map[string]int)
	fieldIndex := 1
	for _, path := range paths {
		if len(path) == 0 {
			continue
		}
		regex = regex + "/"
		if strings.HasPrefix(path, ":") {
			regex = regex + "([^\\/:]+)"
			fields[path[1:]] = fieldIndex
			fieldIndex++
		} else {
			regex = regex + path
		}
	}
	return fields, regex
}
