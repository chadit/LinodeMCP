package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

func tagsValueFromToolArg(rawTags any) ([]string, string) {
	switch tags := rawTags.(type) {
	case string:
		tagsText := strings.TrimSpace(tags)

		var values []string
		if err := json.Unmarshal([]byte(tagsText), &values); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrTagsMustBeJSONStringArray, err).Error()
		}

		if values == nil {
			return nil, ErrTagsMustBeJSONStringArray.Error()
		}

		return normalizeTags(values)
	case []string:
		return normalizeTags(tags)
	case []any:
		values := make([]string, 0, len(tags))
		for _, tag := range tags {
			tagText, ok := tag.(string)
			if !ok {
				return nil, ErrTagsMustBeJSONStringArray.Error()
			}

			values = append(values, tagText)
		}

		return normalizeTags(values)
	default:
		return nil, ErrTagsMustBeJSONStringArray.Error()
	}
}

func normalizeTags(values []string) ([]string, string) {
	normalized := make([]string, len(values))
	for index, value := range values {
		normalized[index] = strings.TrimSpace(value)
		if normalized[index] == "" {
			return nil, ErrTagsEntriesNonEmpty.Error()
		}
	}

	return normalized, ""
}
