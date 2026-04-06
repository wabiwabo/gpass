package sortx

import "testing"

type item struct {
	Name string
	Age  int
}

func TestBy(t *testing.T) {
	items := []item{{"C", 30}, {"A", 10}, {"B", 20}}
	By(items, func(i item) string { return i.Name })

	if items[0].Name != "A" || items[1].Name != "B" || items[2].Name != "C" {
		t.Errorf("By name = %v", items)
	}
}

func TestBy_Int(t *testing.T) {
	items := []item{{"C", 30}, {"A", 10}, {"B", 20}}
	By(items, func(i item) int { return i.Age })

	if items[0].Age != 10 || items[1].Age != 20 || items[2].Age != 30 {
		t.Errorf("By age = %v", items)
	}
}

func TestByDesc(t *testing.T) {
	items := []item{{"A", 10}, {"C", 30}, {"B", 20}}
	ByDesc(items, func(i item) int { return i.Age })

	if items[0].Age != 30 || items[1].Age != 20 || items[2].Age != 10 {
		t.Errorf("ByDesc = %v", items)
	}
}

func TestStableBy(t *testing.T) {
	items := []item{{"A", 10}, {"B", 10}, {"C", 10}}
	StableBy(items, func(i item) int { return i.Age })

	// Same age, original order preserved
	if items[0].Name != "A" || items[1].Name != "B" || items[2].Name != "C" {
		t.Errorf("StableBy = %v", items)
	}
}

func TestTopN(t *testing.T) {
	items := []item{{"C", 30}, {"A", 10}, {"B", 20}, {"D", 5}}
	top := TopN(items, 2, func(i item) int { return i.Age })

	if len(top) != 2 {
		t.Fatalf("len = %d", len(top))
	}
	if top[0].Age != 5 || top[1].Age != 10 {
		t.Errorf("TopN = %v", top)
	}
}

func TestTopN_MoreThanLen(t *testing.T) {
	items := []item{{"A", 10}}
	top := TopN(items, 5, func(i item) int { return i.Age })
	if len(top) != 1 {
		t.Errorf("len = %d", len(top))
	}
}

func TestTopN_Zero(t *testing.T) {
	items := []item{{"A", 10}}
	if TopN(items, 0, func(i item) int { return i.Age }) != nil {
		t.Error("0 should return nil")
	}
}

func TestBottomN(t *testing.T) {
	items := []item{{"A", 10}, {"B", 20}, {"C", 30}}
	bottom := BottomN(items, 2, func(i item) int { return i.Age })

	if len(bottom) != 2 {
		t.Fatalf("len = %d", len(bottom))
	}
	if bottom[0].Age != 30 || bottom[1].Age != 20 {
		t.Errorf("BottomN = %v", bottom)
	}
}

func TestIsSorted(t *testing.T) {
	sorted := []item{{"A", 10}, {"B", 20}, {"C", 30}}
	if !IsSorted(sorted, func(i item) int { return i.Age }) {
		t.Error("should be sorted")
	}

	unsorted := []item{{"A", 30}, {"B", 10}}
	if IsSorted(unsorted, func(i item) int { return i.Age }) {
		t.Error("should not be sorted")
	}

	// Empty and single element
	if !IsSorted([]item{}, func(i item) int { return i.Age }) {
		t.Error("empty should be sorted")
	}
	if !IsSorted([]item{{"A", 1}}, func(i item) int { return i.Age }) {
		t.Error("single should be sorted")
	}
}

func TestBy_Float(t *testing.T) {
	type scored struct {
		Name  string
		Score float64
	}
	items := []scored{{"C", 3.5}, {"A", 1.2}, {"B", 2.8}}
	By(items, func(s scored) float64 { return s.Score })
	if items[0].Name != "A" {
		t.Errorf("float sort: %v", items)
	}
}
