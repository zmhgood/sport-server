package pagination

import "testing"

func TestNormalize(t *testing.T) {
	max := int(^uint(0) >> 1)
	for _, tc := range []struct{ page, size, fallback, wantPage, wantSize int }{
		{0, 0, 10, 1, 10}, {-1, -1, 20, 1, 20}, {1, 100, 10, 1, 100}, {1, 101, 10, 1, 10},
		{max, 100, 10, max / 100, 100}, {max, 1, 10, max, 1},
	} {
		page, size := Normalize(tc.page, tc.size, tc.fallback)
		if page != tc.wantPage || size != tc.wantSize || (page-1)*size < 0 {
			t.Fatalf("bad normalized bounds: %d/%d", page, size)
		}
	}
}

func TestBounds(t *testing.T) {
	max := int(^uint(0) >> 1)
	for _, tc := range []struct{ page, size, total, start, end int }{
		{1, 10, 0, 0, 0}, {1, 10, 25, 0, 10}, {2, 10, 25, 10, 20}, {3, 10, 25, 20, 25},
		{4, 10, 25, 25, 25}, {max, 100, 25, 25, 25}, {1, 10, max, 0, 10}, {-1, 0, 5, 0, 5},
	} {
		start, end := Bounds(tc.page, tc.size, tc.total)
		if start != tc.start || end != tc.end {
			t.Fatalf("bad range: %d:%d", start, end)
		}
	}
}
