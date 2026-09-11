package objectstorage

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
)

// Filters implements the aws s3 --exclude/--include semantics: everything is
// included by default, rules apply in the order given on the command line and
// the last matching rule wins. Patterns are shell globs where `*` also
// matches `/` (fnmatch-style), evaluated against the path relative to the
// source directory or prefix.
type Filters struct {
	rules []filterRule
}

type filterRule struct {
	include bool
	pattern string
	re      *regexp.Regexp
}

// Add appends a rule; returns an error for an invalid pattern.
func (f *Filters) Add(include bool, pattern string) error {
	re, err := globToRegexp(pattern)
	if err != nil {
		return err
	}
	f.rules = append(f.rules, filterRule{include: include, pattern: pattern, re: re})
	return nil
}

// Empty reports whether no rules were given.
func (f *Filters) Empty() bool { return f == nil || len(f.rules) == 0 }

// Include decides whether relPath passes the filters.
func (f *Filters) Include(relPath string) bool {
	if f == nil {
		return true
	}
	include := true
	for _, r := range f.rules {
		if r.re.MatchString(relPath) {
			include = r.include
		}
	}
	return include
}

// Describe lists the rules (for --dry-run/--debug).
func (f *Filters) Describe() string {
	if f.Empty() {
		return "(no filters)"
	}
	parts := make([]string, 0, len(f.rules))
	for _, r := range f.rules {
		kind := "exclude"
		if r.include {
			kind = "include"
		}
		parts = append(parts, fmt.Sprintf("--%s %q", kind, r.pattern))
	}
	return strings.Join(parts, " ")
}

// globToRegexp translates a fnmatch-style pattern into an anchored regexp.
func globToRegexp(pattern string) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString(".")
		case '[':
			j := strings.IndexByte(pattern[i:], ']')
			if j < 0 {
				sb.WriteString(regexp.QuoteMeta(string(c)))
				continue
			}
			class := pattern[i+1 : i+j]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			sb.WriteString("[" + strings.ReplaceAll(class, `\`, `\\`) + "]")
			i += j
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	sb.WriteString("$")
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}
	return re, nil
}

// FilterFlags registers --exclude and --include on a flag set, preserving the
// relative order in which they appear (cobra's per-flag slices cannot).
func FilterFlags(fs *pflag.FlagSet, f *Filters) {
	fs.Var(&filterValue{f: f, include: false}, "exclude", "exclude paths matching this pattern (repeatable; rules apply in order, last match wins)")
	fs.Var(&filterValue{f: f, include: true}, "include", "re-include paths matching this pattern after an --exclude (repeatable)")
}

type filterValue struct {
	f       *Filters
	include bool
}

func (v *filterValue) String() string { return "" }
func (v *filterValue) Type() string   { return "pattern" }
func (v *filterValue) Set(s string) error {
	return v.f.Add(v.include, s)
}
