package rules

import (
	"regexp"
	"strings"
)

type keyMatcher interface {
	getPrefix() string
	match(string) (keyMatch, bool)
	getPattern() string
	getPrefixesWithConstraints(constraints map[string]constraint) []string
}

type regexKeyMatcher struct {
	regex    *regexp.Regexp
	fieldMap map[string]int
	pattern  string
}

type keyMatch interface {
	// GetAttribute usage should be replaced with FindAttribute
	GetAttribute(name string) *string
	FindAttribute(name string) (string, bool)
	Format(pattern string) string
	names() []string
}

func (rkm *regexKeyMatcher) getPrefix() string {
	end := strings.Index(rkm.pattern, ":")
	if end == -1 {
		end = len(rkm.pattern)
	}
	return rkm.pattern[0:end]
}

func (rkm *regexKeyMatcher) getPrefixesWithConstraints(constraints map[string]constraint) []string {
	out := []string{}
	firstColon := strings.Index(rkm.pattern, ":")
	if firstColon == -1 {
		out = append(out, rkm.getPrefix())
	} else {
		end := strings.Index(rkm.pattern[firstColon:], "/")
		if end == -1 {
			end = len(rkm.pattern)
		} else {
			end = firstColon + end
		}
		attrName := rkm.pattern[firstColon+1 : end]
		constr, ok := constraints[attrName]
		if !ok {
			out = append(out, rkm.getPrefix())
		} else {
			outPtr := &out
			buildPrefixesFromConstraint(rkm.pattern[:firstColon]+constr.prefix, 0, constr, outPtr)
			out = *outPtr
		}
	}
	return out
}

func buildPrefixesFromConstraint(base string, index int, constr constraint, prefixes *[]string) {
	myChars := constr.chars[index]
	if index+1 == len(constr.chars) {
		// Last set
		for _, char := range myChars {
			newPrefixes := append(*prefixes, base+string(char))
			*prefixes = newPrefixes
		}
	} else {
		for _, char := range myChars {
			newBase := base + string(char)
			buildPrefixesFromConstraint(newBase, index+1, constr, prefixes)
		}
	}
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

func (m *regexKeyMatch) FindAttribute(name string) (string, bool) {
	index, ok := m.fieldMap[name]
	if !ok {
		return "", false
	}
	result := m.matchStrings[index]
	return result, true
}

func (m *regexKeyMatch) names() []string {
	names := make([]string, 0, len(m.fieldMap))
	for name := range m.fieldMap {
		names = append(names, name)
	}
	return names
}

func (m *regexKeyMatch) Format(pattern string) string {
	return FormatWithAttributes(pattern, m)
}

// FormatWithAttributes applied the specified attributes to the
// provided path.
func FormatWithAttributes(pattern string, m Attributes) string {
	result, _ := formatPath(pattern, m)
	return result
}

type finderWrapper struct{ Attributes }

func (f finderWrapper) FindAttribute(s string) (string, bool) {
	if ptr := f.GetAttribute(s); ptr != nil {
		return *ptr, true
	}

	return "", false
}

func formatPath(pattern string, m Attributes) (string, bool) {
	sb := new(strings.Builder)
	// If the formatted string can fit into 2.5x the length of the pattern
	// (and mapAttributes is the attribute implementation used)
	// this will be the only allocation
	sb.Grow(2*len(pattern) + (len(pattern) / 2))

	var finder AttributeFinder
	if f, ok := m.(AttributeFinder); ok {
		finder = f
	} else {
		finder = finderWrapper{m}
	}

	allFound := true
	var segment string
	for found := true; found; {
		segment, pattern, found = strings.Cut(pattern, "/")
		switch {
		case segment == "":
		case strings.HasPrefix(segment, ":"):
			sb.WriteByte('/')
			if attr, ok := finder.FindAttribute(segment[1:]); ok {
				sb.WriteString(attr)
			} else {
				allFound = false
				sb.WriteString(segment)
			}

		default:
			sb.WriteByte('/')
			sb.WriteString(segment)
		}
	}
	return sb.String(), allFound
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

// NewAttributes provides a map-based Attributes instance,
// for instance for testing callbacks.
func NewAttributes(values map[string]string) Attributes {
	return &mapAttributes{values: values}
}

type mapAttributes struct {
	values map[string]string
}

func (ma *mapAttributes) GetAttribute(key string) *string {
	value, ok := ma.values[key]
	if !ok {
		return nil
	}
	return &value
}

func (ma *mapAttributes) FindAttribute(key string) (string, bool) {
	value, ok := ma.values[key]
	return value, ok
}

func (ma *mapAttributes) Format(path string) string {
	return FormatWithAttributes(path, ma)
}

func (ma *mapAttributes) names() []string {
	names := make([]string, 0, len(ma.values))
	for key := range ma.values {
		names = append(names, key)
	}
	return names
}

func parsePath(pattern string) (map[string]int, string) {
	paths := strings.Split(pattern, "/")
	regex := strings.Builder{}
	fields := make(map[string]int)
	fieldIndex := 1
	for _, path := range paths {
		if len(path) == 0 {
			continue
		}
		regex.WriteString("/")
		if strings.HasPrefix(path, ":") {
			regex.WriteString("([^\\/:]+)")
			fields[path[1:]] = fieldIndex
			fieldIndex++
		} else {
			regex.WriteString(path)
		}
	}
	return fields, regex.String()
}
