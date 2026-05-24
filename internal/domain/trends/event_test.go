package trends

import (
	"errors"
	"testing"
)

func TestNewSearchQueryNormalizes(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"iPhone 15", "iphone 15"},
		{"  iphone   15  ", "iphone 15"},
		{"IPHONE\t15\n", "iphone 15"},
		{"sneakers", "sneakers"},
	}
	for _, tc := range cases {
		q, err := NewSearchQuery(tc.in)
		if err != nil {
			t.Errorf("NewSearchQuery(%q) returned error: %v", tc.in, err)
			continue
		}
		if q.String() != tc.want {
			t.Errorf("NewSearchQuery(%q) = %q, want %q", tc.in, q.String(), tc.want)
		}
	}
}

func TestNewSearchQueryRejectsEmpty(t *testing.T) {
	cases := []string{"", "   ", "\t\n"}
	for _, in := range cases {
		_, err := NewSearchQuery(in)
		if !errors.Is(err, ErrEmptyQuery) {
			t.Errorf("NewSearchQuery(%q): expected ErrEmptyQuery, got %v", in, err)
		}
	}
}

func TestSearchQueryZeroIsInvalid(t *testing.T) {
	var q SearchQuery
	if !q.IsZero() {
		t.Fatal("zero SearchQuery should be IsZero()")
	}
}
