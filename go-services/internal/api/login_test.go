package api

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// Unit tests: pure functions, no database or HTTP.

func TestLooksLikeMD5(t *testing.T) {
	tests := []struct {
		name   string
		stored string
		want   bool
	}{
		{"md5 of 'password'", "5f4dcc3b5aa765d61d8327deb882cf99", true},
		{"uppercase hex is still hex", "5F4DCC3B5AA765D61D8327DEB882CF99", true},
		{"too short", "5f4dcc3b", false},
		{"32 chars but not hex", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", false},
		{"bcrypt hash", "$2a$10$abcdefghijklmnopqrstuuJ1Q0lN9yQkB2dQ8m1u5yQ8m1u5yQ8m", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeMD5(tt.stored); got != tt.want {
				t.Errorf("looksLikeMD5(%q) = %v, want %v", tt.stored, got, tt.want)
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	// MinCost keeps the test fast; the cost doesn't change correctness.
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		password string
		stored   string
		want     bool
	}{
		{"legacy md5 match", "password", "5f4dcc3b5aa765d61d8327deb882cf99", true},
		{"legacy md5 mismatch", "wrong", "5f4dcc3b5aa765d61d8327deb882cf99", false},
		{"bcrypt match", "s3cret", string(bcryptHash), true},
		{"bcrypt mismatch", "wrong", string(bcryptHash), false},
		{"empty password vs bcrypt", "", string(bcryptHash), false},
		{"garbage stored value", "password", "not-a-hash", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkPassword(tt.password, tt.stored); got != tt.want {
				t.Errorf("checkPassword(%q, %q) = %v, want %v", tt.password, tt.stored, got, tt.want)
			}
		})
	}
}
