package sortslice

import (
	"cmp"
	"testing"
	"time"
)

func TestAscInt(t *testing.T) {
	s := []int{5, 3, 1, 4, 2}
	Asc(s)
	want := []int{1, 2, 3, 4, 5}
	for i := range s {
		if s[i] != want[i] {
			t.Errorf("index %d: got %d, want %d", i, s[i], want[i])
		}
	}
}

func TestAscString(t *testing.T) {
	s := []string{"banana", "apple", "cherry"}
	Asc(s)
	want := []string{"apple", "banana", "cherry"}
	for i := range s {
		if s[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, s[i], want[i])
		}
	}
}

func TestAscEmpty(t *testing.T) {
	var s []int
	Asc(s) // should not panic
}

func TestAscSingle(t *testing.T) {
	s := []int{42}
	Asc(s)
	if s[0] != 42 {
		t.Errorf("got %d, want 42", s[0])
	}
}

func TestDescInt(t *testing.T) {
	s := []int{1, 2, 3, 4, 5}
	Desc(s)
	want := []int{5, 4, 3, 2, 1}
	for i := range s {
		if s[i] != want[i] {
			t.Errorf("index %d: got %d, want %d", i, s[i], want[i])
		}
	}
}

func TestDescFloat(t *testing.T) {
	s := []float64{1.1, 3.3, 2.2}
	Desc(s)
	if s[0] != 3.3 || s[1] != 2.2 || s[2] != 1.1 {
		t.Errorf("got %v, want [3.3 2.2 1.1]", s)
	}
}

type user struct {
	Name string
	Age  int
}

func TestBy(t *testing.T) {
	users := []user{
		{"Charlie", 30},
		{"Alice", 25},
		{"Bob", 28},
	}
	By(users, func(u user) string { return u.Name })
	if users[0].Name != "Alice" || users[1].Name != "Bob" || users[2].Name != "Charlie" {
		t.Errorf("got %v", users)
	}
}

func TestByAge(t *testing.T) {
	users := []user{
		{"Charlie", 30},
		{"Alice", 25},
		{"Bob", 28},
	}
	By(users, func(u user) int { return u.Age })
	if users[0].Age != 25 || users[1].Age != 28 || users[2].Age != 30 {
		t.Errorf("got %v", users)
	}
}

func TestByDesc(t *testing.T) {
	users := []user{
		{"Alice", 25},
		{"Bob", 28},
		{"Charlie", 30},
	}
	ByDesc(users, func(u user) int { return u.Age })
	if users[0].Age != 30 || users[1].Age != 28 || users[2].Age != 25 {
		t.Errorf("got %v", users)
	}
}

func TestByTime(t *testing.T) {
	t1 := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	type event struct {
		Name string
		At   time.Time
	}
	events := []event{
		{"C", t1}, {"A", t2}, {"B", t3},
	}
	ByTime(events, func(e event) time.Time { return e.At })
	if events[0].Name != "A" || events[1].Name != "B" || events[2].Name != "C" {
		t.Errorf("got %v", events)
	}
}

func TestByTimeDesc(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	type event struct {
		Name string
		At   time.Time
	}
	events := []event{
		{"A", t1}, {"C", t2}, {"B", t3},
	}
	ByTimeDesc(events, func(e event) time.Time { return e.At })
	if events[0].Name != "C" || events[1].Name != "B" || events[2].Name != "A" {
		t.Errorf("got %v", events)
	}
}

func TestByTimeEqual(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	type item struct{ V int }
	items := []item{{1}, {2}}
	ByTime(items, func(i item) time.Time { return ts })
	// Should not panic; order may be either way
	if len(items) != 2 {
		t.Error("length changed")
	}
}

func TestCaseInsensitive(t *testing.T) {
	s := []string{"Banana", "apple", "Cherry", "APRICOT"}
	CaseInsensitive(s)
	// Expected: apple, APRICOT, Banana, Cherry
	if s[0] != "apple" || s[1] != "APRICOT" || s[2] != "Banana" || s[3] != "Cherry" {
		t.Errorf("got %v", s)
	}
}

func TestStable(t *testing.T) {
	type item struct {
		Key   int
		Order int
	}
	items := []item{
		{1, 1}, {2, 2}, {1, 3}, {2, 4}, {1, 5},
	}
	Stable(items, func(a, b item) int {
		return cmp.Compare(a.Key, b.Key)
	})
	// All key=1 items should maintain relative order
	var ones []int
	for _, it := range items {
		if it.Key == 1 {
			ones = append(ones, it.Order)
		}
	}
	if ones[0] != 1 || ones[1] != 3 || ones[2] != 5 {
		t.Errorf("stable sort violated: %v", ones)
	}
}

func TestIsSorted(t *testing.T) {
	tests := []struct {
		name string
		s    []int
		want bool
	}{
		{"sorted", []int{1, 2, 3, 4, 5}, true},
		{"unsorted", []int{5, 3, 1}, false},
		{"empty", []int{}, true},
		{"single", []int{42}, true},
		{"equal", []int{1, 1, 1}, true},
		{"almost", []int{1, 2, 3, 2}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSorted(tt.s); got != tt.want {
				t.Errorf("IsSorted(%v) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name string
		s    []int
		want []int
	}{
		{"with_dups", []int{1, 1, 2, 2, 3, 3}, []int{1, 2, 3}},
		{"no_dups", []int{1, 2, 3}, []int{1, 2, 3}},
		{"all_same", []int{5, 5, 5}, []int{5}},
		{"empty", []int{}, []int{}},
		{"single", []int{1}, []int{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Unique(tt.s)
			if len(got) != len(tt.want) {
				t.Fatalf("length: got %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	s := []string{"a", "a", "b", "c", "c"}
	got := Unique(s)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("length: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAscThenUnique(t *testing.T) {
	s := []int{3, 1, 2, 1, 3, 2}
	Asc(s)
	got := Unique(s)
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("length: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("index %d: got %d, want %d", i, got[i], want[i])
		}
	}
}
