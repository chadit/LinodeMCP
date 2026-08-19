package tools

import (
	"regexp"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	errPlacementGroupLabelPattern = "label must start and end with an alphanumeric character and contain only " +
		"alphanumeric characters, hyphens, underscores, or periods"
)

// placementGroupLabelPattern mirrors Python's _LABEL_PATTERN so both languages
// reject a label that does not start and end alphanumeric or uses characters
// other than letters, digits, hyphens, underscores, or periods.
var placementGroupLabelPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

// PlacementGroupLabelMessage answers the sentence a label Linode will not accept
// is refused under, or "" for one it will. It is exported because the placement
// group create and update tools both check their label from a hook that lives
// outside this package.
func PlacementGroupLabelMessage(label string) string {
	if !placementGroupLabelPattern.MatchString(label) {
		return errPlacementGroupLabelPattern
	}

	return ""
}

// ParsePlacementGroupLinodes reads the Linode ids a placement group membership
// call carries. It is exported because both membership tools check them from a
// hook, which lives outside this package.
func ParsePlacementGroupLinodes(request *mcp.CallToolRequest) ([]int, string) {
	raw, exists := request.GetArguments()["linodes"]
	if !exists {
		return nil, ErrPlacementGroupLinodesRequired.Error()
	}

	rawLinodes, ok := raw.([]any)
	if !ok {
		return nil, ErrPlacementGroupLinodesJSON.Error()
	}

	if len(rawLinodes) == 0 {
		return nil, ErrPlacementGroupLinodesEmpty.Error()
	}

	linodes := make([]int, 0, len(rawLinodes))
	seen := make(map[int]struct{}, len(rawLinodes))

	for _, rawLinode := range rawLinodes {
		linodeID, ok := numberArgToInt(rawLinode)
		if !ok || linodeID <= 0 {
			return nil, ErrPlacementGroupLinodesPositive.Error()
		}

		if _, exists := seen[linodeID]; exists {
			return nil, ErrPlacementGroupLinodesDuplicate.Error()
		}

		seen[linodeID] = struct{}{}
		linodes = append(linodes, linodeID)
	}

	return linodes, ""
}
