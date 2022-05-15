package tools

import "strings"

func ReplaceVersion(s string) string {
	s1 := strings.ReplaceAll(s, ".", "-")
	s2 := strings.ReplaceAll(s1, "_", "-")
	return s2
}
