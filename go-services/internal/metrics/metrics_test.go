package metrics

import "testing"

func TestRankBucket(t *testing.T) {
	cases := map[int]string{0: "unknown", -1: "unknown", 1: "1", 3: "3", 4: "4-10", 10: "4-10", 11: "11+", 200: "11+"}
	for rank, want := range cases {
		if got := RankBucket(rank); got != want {
			t.Errorf("RankBucket(%d) = %q, want %q", rank, got, want)
		}
	}
}
