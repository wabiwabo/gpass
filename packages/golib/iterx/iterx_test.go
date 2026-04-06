package iterx

import "testing"

func TestRange(t *testing.T) {
	r := Range(0, 5)
	if len(r) != 5 { t.Fatalf("len = %d", len(r)) }
	if r[0] != 0 || r[4] != 4 { t.Error("values") }
}

func TestRange_Empty(t *testing.T) {
	if Range(5, 3) != nil { t.Error("reversed") }
	if Range(3, 3) != nil { t.Error("equal") }
}

func TestRepeat(t *testing.T) {
	r := Repeat("x", 3)
	if len(r) != 3 { t.Fatalf("len = %d", len(r)) }
	for _, v := range r {
		if v != "x" { t.Error("value") }
	}
}

func TestRepeat_Zero(t *testing.T) {
	if Repeat("x", 0) != nil { t.Error("zero") }
}

func TestZip(t *testing.T) {
	z := Zip([]int{1, 2, 3}, []string{"a", "b"})
	if len(z) != 2 { t.Fatalf("len = %d", len(z)) }
	if z[0].First != 1 || z[0].Second != "a" { t.Error("[0]") }
	if z[1].First != 2 || z[1].Second != "b" { t.Error("[1]") }
}

func TestEnumerate(t *testing.T) {
	e := Enumerate([]string{"a", "b", "c"})
	if len(e) != 3 { t.Fatalf("len = %d", len(e)) }
	if e[0].Index != 0 || e[0].Value != "a" { t.Error("[0]") }
	if e[2].Index != 2 || e[2].Value != "c" { t.Error("[2]") }
}

func TestGroupBy(t *testing.T) {
	type item struct{ Name string; Cat string }
	items := []item{{"a", "x"}, {"b", "y"}, {"c", "x"}}
	groups := GroupBy(items, func(i item) string { return i.Cat })

	if len(groups["x"]) != 2 { t.Errorf("x = %d", len(groups["x"])) }
	if len(groups["y"]) != 1 { t.Errorf("y = %d", len(groups["y"])) }
}

func TestCountBy(t *testing.T) {
	words := []string{"hello", "world", "hello", "go", "hello"}
	counts := CountBy(words, func(w string) string { return w })
	if counts["hello"] != 3 { t.Errorf("hello = %d", counts["hello"]) }
}

func TestAny(t *testing.T) {
	if !Any([]int{1, 2, 3}, func(n int) bool { return n > 2 }) { t.Error("should match") }
	if Any([]int{1, 2, 3}, func(n int) bool { return n > 5 }) { t.Error("should not match") }
}

func TestAll(t *testing.T) {
	if !All([]int{2, 4, 6}, func(n int) bool { return n%2 == 0 }) { t.Error("all even") }
	if All([]int{2, 3, 6}, func(n int) bool { return n%2 == 0 }) { t.Error("not all even") }
}

func TestNone(t *testing.T) {
	if !None([]int{1, 3, 5}, func(n int) bool { return n%2 == 0 }) { t.Error("none even") }
	if None([]int{1, 2, 5}, func(n int) bool { return n%2 == 0 }) { t.Error("has even") }
}

func TestSum(t *testing.T) {
	if Sum([]int{1, 2, 3, 4}) != 10 { t.Error("int sum") }
	if Sum([]float64{1.5, 2.5}) != 4.0 { t.Error("float sum") }
}

func TestMin(t *testing.T) {
	if Min([]int{3, 1, 4, 1, 5}) != 1 { t.Error("min") }
	if Min([]string{"c", "a", "b"}) != "a" { t.Error("string min") }
}

func TestMax(t *testing.T) {
	if Max([]int{3, 1, 4, 1, 5}) != 5 { t.Error("max") }
}

func TestMin_Empty(t *testing.T) {
	if Min([]int{}) != 0 { t.Error("empty") }
}
