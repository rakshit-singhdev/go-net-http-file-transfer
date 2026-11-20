package helpers

import "strconv"

func ParseUploadLimit(value string, fallback int64) int64 {
	limit, err := strconv.ParseInt(value, 10, 64)
	if err != nil || limit <= 0 {
		return fallback
	}

	return limit
}