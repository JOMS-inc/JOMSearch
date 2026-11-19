package searchlog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLogThenRank(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "searches.log") // dir is created
	logger, f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	Log(logger, "Go", "en", 3)
	Log(logger, "kubernetes", "en", 0)
	Log(logger, " go ", "en", 4) // same term as "Go"
	Log(logger, "docker", "en", 0)
	Log(logger, "kubernetes", "en", 0)
	Log(logger, "go", "da", 0) // other language = other term
	_ = f.Close()

	r, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close() //nolint:errcheck // read-only file
	got, err := Rank(r)
	if err != nil {
		t.Fatal(err)
	}

	// No results first (most searched first), then the rest.
	want := []Term{
		{"kubernetes", "en", 2, 0},
		{"docker", "en", 1, 0},
		{"go", "da", 1, 0},
		{"go", "en", 2, 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Rank =\n%+v\nwant\n%+v", got, want)
	}
}

func TestRankSkipsOtherLines(t *testing.T) {
	in := strings.Join([]string{
		`{"msg":"listening","port":"8080"}`,
		`not json at all`,
		`{"msg":"search","query":"  ","language":"en","results":0}`,
		`{"msg":"search","query":"sqlite","language":"en","results":1}`,
		`{"msg":"search","query":"half writ`,
	}, "\n")
	got, err := Rank(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Query != "sqlite" {
		t.Errorf("Rank = %+v, want only sqlite", got)
	}
}
