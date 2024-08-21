package float

import (
	"regexp"
)

func extractMountPaths(input string) []string {
	re := regexp.MustCompile(`(--dataVolume)\s+(\[(?:[^\]]*)\]s3://[^:\s]+:[^\s']+)`)
	matches := re.FindAllStringSubmatch(input, -1)
	var result []string
	for _, match := range matches {
		if len(match) >= 3 {
			result = append(result, match[1], match[2])
		}
	}
	return result
}
