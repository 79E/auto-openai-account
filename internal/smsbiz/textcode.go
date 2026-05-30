package smsbiz

import "regexp"

var textCodeRE = regexp.MustCompile(`(^|\D)(\d{6})(\D|$)`)

func ExtractCodeFromText(body string) string {
	matches := textCodeRE.FindStringSubmatch(body)
	if len(matches) < 3 {
		return ""
	}
	return matches[2]
}
