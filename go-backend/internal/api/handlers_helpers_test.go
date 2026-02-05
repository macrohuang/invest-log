package api

import "testing"

func TestNormalizeLimitOffset(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{name: "defaults", limit: 0, offset: 0, wantLimit: 100, wantOffset: 0},
		{name: "negative offset", limit: 25, offset: -5, wantLimit: 25, wantOffset: 0},
		{name: "negative limit", limit: -1, offset: 3, wantLimit: 100, wantOffset: 3},
		{name: "pass through", limit: 10, offset: 2, wantLimit: 10, wantOffset: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset := normalizeLimitOffset(tt.limit, tt.offset)
			if limit != tt.wantLimit || offset != tt.wantOffset {
				t.Fatalf("expected (%d, %d), got (%d, %d)", tt.wantLimit, tt.wantOffset, limit, offset)
			}
		})
	}
}
