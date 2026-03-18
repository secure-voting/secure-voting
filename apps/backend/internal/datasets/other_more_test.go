package datasets

import (
	"reflect"
	"sort"
	"testing"
)

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestBallotsToJSON_More(t *testing.T) {
	in := []BallotDoc{
		{
			VoterRef: "v1",
			Approval: []string{"c1", "c2"},
		},
		{
			VoterRef: "v2",
			Ranking:  []string{"c3", "c1"},
		},
		{
			VoterRef: "v3",
			Scores:   map[string]int{"c1": 5, "c2": 3},
		},
		{
			VoterRef: "v4",
		},
	}

	got := ballotsToJSON(in)

	if len(got) != 4 {
		t.Fatalf("expected 4 items, got %d", len(got))
	}

	if !reflect.DeepEqual(got[0]["approval"], []string{"c1", "c2"}) {
		t.Fatalf("unexpected approval: %#v", got[0]["approval"])
	}
	if !reflect.DeepEqual(got[1]["ranking"], []string{"c3", "c1"}) {
		t.Fatalf("unexpected ranking: %#v", got[1]["ranking"])
	}
	if !reflect.DeepEqual(got[2]["scores"], map[string]int{"c1": 5, "c2": 3}) {
		t.Fatalf("unexpected scores: %#v", got[2]["scores"])
	}
	if _, ok := got[3]["approval"]; ok {
		t.Fatalf("unexpected approval key in empty ballot: %#v", got[3])
	}
	if got[3]["voter_ref"] != "v4" {
		t.Fatalf("unexpected voter_ref: %#v", got[3]["voter_ref"])
	}
}

func TestToAny_More(t *testing.T) {
	in := []int{1, 2, 3}
	got := toAny(in)

	if len(got) != 3 {
		t.Fatalf("expected len=3, got %d", len(got))
	}
	for i, v := range got {
		n, ok := v.(int)
		if !ok {
			t.Fatalf("item %d is not int: %#v", i, v)
		}
		if n != in[i] {
			t.Fatalf("item %d = %d, want %d", i, n, in[i])
		}
	}
}

func TestLCGAndShuffle_More(t *testing.T) {
	r1 := newLCG(42)
	r2 := newLCG(42)

	if r1.next() != r2.next() {
		t.Fatal("same seed must produce same first value")
	}
	if r1.next() != r2.next() {
		t.Fatal("same seed must produce same second value")
	}

	in := []string{"a", "b", "c", "d", "e"}
	r3 := newLCG(7)
	got := shuffle(r3, in)

	if len(got) != len(in) {
		t.Fatalf("shuffle len mismatch: got=%d want=%d", len(got), len(in))
	}
	if !reflect.DeepEqual(sortedCopy(got), sortedCopy(in)) {
		t.Fatalf("shuffle must preserve elements: got=%v want=%v", got, in)
	}
	if &got[0] == &in[0] && len(in) > 0 {
		t.Fatal("shuffle must return a copied slice")
	}
}

func TestPickSubset_More(t *testing.T) {
	in := []string{"a", "b", "c", "d"}

	r := newLCG(11)
	got0 := pickSubset(r, in, 0)
	if len(got0) != 0 {
		t.Fatalf("expected empty subset, got %v", got0)
	}

	r = newLCG(11)
	gotAll := pickSubset(r, in, len(in))
	if !reflect.DeepEqual(gotAll, in) {
		t.Fatalf("expected full copy, got %v want %v", gotAll, in)
	}

	r = newLCG(11)
	gotSome := pickSubset(r, in, 2)
	if len(gotSome) != 2 {
		t.Fatalf("expected subset of len 2, got %d", len(gotSome))
	}

	for _, v := range gotSome {
		found := false
		for _, x := range in {
			if x == v {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unexpected subset element %q", v)
		}
	}
}

func TestItoa_More(t *testing.T) {
	cases := map[int]string{
		0:   "0",
		1:   "1",
		15:  "15",
		-7:  "-7",
		123: "123",
	}

	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}
