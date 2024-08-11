package float

import (
	"regexp"
)

func extractMountPaths(input string) []string {
	re := regexp.MustCompile(`--dataVolume\s+\[(?:[^\]]*)\]s3://[^:\s]+:[^\s']+`)
	matches := re.FindAllString(input, -1)
	return matches
}
