package internal

import "strings"

func JoinNewLines(s ...string) string {
	return strings.Join(s, "\n")
}
