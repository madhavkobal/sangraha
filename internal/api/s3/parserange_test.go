package s3

import (
	"testing"
)

func TestParseContentRange(t *testing.T) {
	tests := []struct {
		header     string
		wantOffset int64
		wantLength int64
		wantErr    bool
	}{
		{"", 0, -1, false},
		{"bytes=0-1023", 0, 1024, false},
		{"bytes=100-199", 100, 100, false},
		{"bytes=0-", 0, -1, false},
		{"bytes=-", 0, -1, false},
		{"bytes=notanumber-100", 0, 0, true},
		{"bytes=0-notanumber", 0, 0, true},
	}
	for _, tc := range tests {
		offset, length, err := parseContentRange(tc.header)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseContentRange(%q): expected error, got nil", tc.header)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseContentRange(%q): unexpected error: %v", tc.header, err)
			continue
		}
		if offset != tc.wantOffset || length != tc.wantLength {
			t.Errorf("parseContentRange(%q) = (%d, %d); want (%d, %d)",
				tc.header, offset, length, tc.wantOffset, tc.wantLength)
		}
	}
}
