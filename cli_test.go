package cli

import "testing"

func Test_DisplayCols(t *testing.T) {
	clist := [][]string{
		{"a", "bb", "c"},
		{"aa", "b", "cb"},
		{"aaa", "bbbb", "ccccccc"},
	}
	csize := []int{8, 10, 15}
	t.Logf("\n%s\n", TableString(clist, csize, 1))
	t.Logf("\n%s\n", TableString(clist, nil, 1))
}

func indexCompare(a, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i][0] != b[i][0] {
			return false
		}
		if a[i][1] != b[i][1] {
			return false
		}
	}
	return true
}

func Test_SplitIndex(t *testing.T) {
	tests := []struct {
		s string
		r [][2]int
	}{
		{"aaa bb  ccccc      ddddd", [][2]int{{0, 3}, {4, 6}, {8, 13}, {19, 24}}},
		{"", [][2]int{}},
		{"a", [][2]int{{0, 1}}},
	}
	for i, v := range tests {
		r := splitIndex(v.s)
		if !indexCompare(r, v.r) {
			t.Errorf("%d: FAIL expected (%v) != actual (%v)", i, v.r, r)
		}
	}
}
