package glob

import (
	"fmt"

	"github.com/gobwas/glob"
	"github.com/thoas/go-funk"
)

type GlobMatcher struct {
	pattern         string
	globPattern     glob.Glob
	excludes        []string
	excludesPattern []glob.Glob
}

func NewGlobMatcher(pattern string, excludes ...string) *GlobMatcher {
	return &GlobMatcher{
		pattern:         pattern,
		globPattern:     glob.MustCompile(pattern),
		excludes:        excludes,
		excludesPattern: funk.Map(excludes, func(p string) glob.Glob { return glob.MustCompile(p) }).([]glob.Glob),
	}
}

func (m GlobMatcher) Match(value string) (bool, string) {

	// Handle exclusion
	for i, p := range m.excludesPattern {
		if p.Match((value)) {
			return false, fmt.Sprintf("'%s' match exclusion pattern '%s'", value, m.excludes[i])
		}
	}

	if m.pattern != "" && !m.globPattern.Match(value) {
		return false, fmt.Sprintf("'%s' not match with pattern '%s'", value, m.pattern)
	}

	return true, ""
}
