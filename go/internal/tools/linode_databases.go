package tools

func numberArgToInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}

		return int(typed), true
	default:
		return 0, false
	}
}
