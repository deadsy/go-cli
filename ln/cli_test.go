package ln

import (
	"fmt"
	"testing"
)

func Test_DisplayCols(t *testing.T) {
	clist := [][]string{
		{"a", "bb", "c"},
		{"aa", "b", "cb"},
		{"aaa", "bbbb", "ccccccc"},
	}
	csize := []int{8, 10, 15}
	fmt.Printf("%s\n", DisplayCols(clist, csize))
	fmt.Printf("%s\n", DisplayCols(clist, nil))
}
