package builder_test

import (
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

// Tool names the builder tests share as catalog entries and pattern targets.
const (
	toolInstanceList = "linode_instance_list"
	toolInstanceGet  = "linode_instance_get"
	toolDomainList   = "linode_domain_list"
	toolHello        = "hello"

	// Pattern and field values the builder tests reuse across files.
	instanceGlob   = "linode_instance_*"
	envProd        = "prod"
	envDev         = "dev"
	descDNSRead    = "read-only DNS access"
	scopeLinodesRO = "linodes:read_only"
)

// matchCatalog is the tool catalog the pattern tests expand against. It mixes
// two prefixes and one unrelated name so a wildcard that over-matches shows up
// as an extra entry rather than as an equal-length but wrong list.
func matchCatalog() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		{Name: toolInstanceList},
		{Name: toolInstanceGet},
		{Name: toolDomainList},
		{Name: toolHello},
	}
}

// TestMatchPatternsExpandsLiteralsAndWildcards walks the pattern grammar the
// builder promises: literals must hit an existing tool, wildcards go through
// shell-glob matching, and repeated hits collapse to one sorted list.
func TestMatchPatternsExpandsLiteralsAndWildcards(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{
			name:     "nil patterns match nothing",
			patterns: nil,
			want:     []string{},
		},
		{
			name:     "empty pattern string is skipped",
			patterns: []string{""},
			want:     []string{},
		},
		{
			name:     "literal hit returns that one name",
			patterns: []string{toolDomainList},
			want:     []string{toolDomainList},
		},
		{
			name:     "unknown literal contributes nothing",
			patterns: []string{"linode_nope"},
			want:     []string{},
		},
		{
			name:     "prefix wildcard expands to every match",
			patterns: []string{instanceGlob},
			want:     []string{toolInstanceGet, toolInstanceList},
		},
		{
			name:     "bare wildcard expands to the whole catalog",
			patterns: []string{"*"},
			want:     []string{toolHello, toolDomainList, toolInstanceGet, toolInstanceList},
		},
		{
			name:     "overlapping patterns report each name once",
			patterns: []string{instanceGlob, toolInstanceGet},
			want:     []string{toolInstanceGet, toolInstanceList},
		},
		{
			name:     "malformed wildcard matches nothing",
			patterns: []string{"linode_[instance"},
			want:     []string{},
		},
		{
			name:     "results are sorted regardless of pattern order",
			patterns: []string{toolHello, toolDomainList},
			want:     []string{toolHello, toolDomainList},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := builder.MatchPatterns(tt.patterns, matchCatalog())

			if got == nil {
				t.Fatal("MatchPatterns returned nil, want an empty slice")
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchPatterns(%v) = %v, want %v", tt.patterns, got, tt.want)
			}
		})
	}
}

// TestMatchPatternsAgainstEmptyCatalog covers the state a fresh draft is in:
// nothing registered yet, so even a catch-all pattern comes back empty instead
// of nil.
func TestMatchPatternsAgainstEmptyCatalog(t *testing.T) {
	t.Parallel()

	got := builder.MatchPatterns([]string{"*", toolInstanceList}, nil)

	if len(got) != 0 {
		t.Errorf("MatchPatterns against an empty catalog = %v, want no matches", got)
	}
}
