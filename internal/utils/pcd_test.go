package utils

/*

go test -run 'TestComputeMinPCD' -v ./internal/utils -count=1

*/

import "testing"

func TestComputeMinPCD(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{0, 0}, {99, 0}, {100, 2}, {150, 3}, {200, 4},
		{201, 7}, {500, 15}, {501, 21}, {1000, 40}, {1001, 51},
	}
	for _, tc := range cases {
		if got := ComputeMinPCD(tc.n); got != tc.want {
			t.Fatalf("n=%d want=%d got=%d", tc.n, tc.want, got)
		}
	}
}
