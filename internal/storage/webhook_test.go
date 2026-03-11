package storage

import (
	"testing"
)

func TestEventMatchesTarget(t *testing.T) {
	tests := []struct {
		pattern string
		event   EventType
		want    bool
	}{
		{"s3:ObjectCreated:*", EventObjectCreatedPut, true},
		{"s3:ObjectCreated:*", EventObjectCreatedMultipartCompleted, true},
		{"s3:ObjectCreated:*", EventObjectRemovedDelete, false},
		{"s3:ObjectRemoved:*", EventObjectRemovedDelete, true},
		{"s3:ObjectCreated:Put", EventObjectCreatedPut, true},
		{"s3:ObjectCreated:Put", EventObjectCreatedMultipartCompleted, false},
		{"s3:*", EventObjectCreatedPut, true},
		{"s3:*", EventObjectRemovedDelete, true},
	}
	for _, tc := range tests {
		got := matchEventPattern(tc.pattern, string(tc.event))
		if got != tc.want {
			t.Errorf("matchEventPattern(%q, %q) = %v, want %v", tc.pattern, tc.event, got, tc.want)
		}
	}
}
