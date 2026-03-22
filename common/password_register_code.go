package common

import (
	"encoding/json"
	"strings"
)

func NormalizePasswordRegisterCodes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var jsonCodes []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &jsonCodes); err == nil {
			return normalizePasswordRegisterCodesSlice(jsonCodes)
		}
	}

	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	return normalizePasswordRegisterCodesSlice(fields)
}

func normalizePasswordRegisterCodesSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(input))
	codes := make([]string, 0, len(input))
	for _, item := range input {
		code := strings.TrimSpace(item)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return nil
	}
	return codes
}

func UpdatePasswordRegisterCodes(raw string) {
	PasswordRegisterCodes = NormalizePasswordRegisterCodes(raw)
}

func IsPasswordRegisterCodeValid(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	for _, candidate := range PasswordRegisterCodes {
		if candidate == code {
			return true
		}
	}
	return false
}
