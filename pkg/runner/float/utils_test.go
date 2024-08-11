package float

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractMountPaths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "Full input string",
			input: `process {
				executor = 'float'
				errorStrategy = 'retry'
				extra = '--dataVolume [opts=" --cache-dir /mnt/jfs_cache "]jfs://${jfs_private_ip}:6868/1:/mnt/jfs --dataVolume [size=120]:/mnt/jfs_cache --vmPolicy [retryLimit=10,retryInterval=300s] --migratePolicy [disable=true] --dumpMode incremental --snapLocation [mode=rw]s3://cfdx-juicefs-snapshots --dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=rw]s3://bucket-experiments/:/bucket-experiments --dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=r]s3://bucket-research/:/bucket-research --dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=r]s3://bucket-data/:/bucket-data --dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=r]s3://bucket-entry/:/bucket-entry'
			}`,
			expected: []string{
				"--dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=rw]s3://bucket-experiments/:/bucket-experiments",
				"--dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=r]s3://bucket-research/:/bucket-research",
				"--dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=r]s3://bucket-data/:/bucket-data",
				"--dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=r]s3://bucket-entry/:/bucket-entry",
			},
		},
		{
			name:     "Empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "Input with no dataVolume",
			input:    "process { executor = 'float' errorStrategy = 'retry' }",
			expected: []string{},
		},
		{
			name:  "Input with single S3 dataVolume",
			input: "--dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=rw]s3://bucket-experiments/:/bucket-experiments",
			expected: []string{
				"--dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=rw]s3://bucket-experiments/:/bucket-experiments",
			},
		},
		{
			name:  "Input with mixed dataVolumes",
			input: "--dataVolume jfs://example.com:/path --dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=r]s3://cfdx-data/:/cfdx-data",
			expected: []string{
				"--dataVolume [endpoint=s3.us-east-1.amazonaws.com,mode=r]s3://cfdx-data/:/cfdx-data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMountPaths(tt.input)
			if !equalStringSlices(result, tt.expected) {
				t.Errorf("Test case: %s\nexpectMountPaths() =\n%s\nwant\n%s",
					tt.name,
					formatStringSlice(result),
					formatStringSlice(tt.expected))
			}
		})
	}
}

func formatStringSlice(slice []string) string {
	if len(slice) == 0 {
		return "[]"
	}
	return fmt.Sprintf("[\n\t%s\n]", strings.Join(slice, "\n\t"))
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
