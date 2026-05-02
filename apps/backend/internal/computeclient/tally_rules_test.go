package computeclient

import (
	"reflect"
	"testing"
)

func TestNormalizeBallotFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "ranking unchanged",
			in:   "ranking",
			want: "ranking",
		},
		{
			name: "approval unchanged",
			in:   "approval",
			want: "approval",
		},
		{
			name: "score unchanged",
			in:   "score",
			want: "score",
		},
		{
			name: "scoring normalized to score",
			in:   "scoring",
			want: "score",
		},
		{
			name: "trim and lowercase",
			in:   " Scoring ",
			want: "score",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBallotFormat(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeBallotFormat(%q)=%q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUniqueNonEmpty(t *testing.T) {
	got := uniqueNonEmpty([]string{"ranking", "", "score", "ranking", " score "})
	want := []string{"ranking", "score"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueNonEmpty mismatch: got %#v want %#v", got, want)
	}
}
