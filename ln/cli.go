//-----------------------------------------------------------------------------
/*
Command Line Interface

Implements a CLI with:

* hierarchical menus
* command tab completion
* command history
* context sensitive help
* command editing

Notes:

Menu Tuple Format:
  (name, submenu, description) - submenu
  (name, leaf) - leaf command with generic <cr> help
  (name, leaf, help) - leaf command with specific argument help

Help Format:
  (parm, descr)

Leaf Functions:

def leaf_function(ui, args):
 .....

ui: the ui object passed by the application to cli()
args: the argument list from the command line

The general help for a leaf function is the docstring for that function.
*/
//-----------------------------------------------------------------------------

package ln

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

//-----------------------------------------------------------------------------

// Each leaf function is called with an object with this interface.
type UI interface {
	Put(s string)
}

//-----------------------------------------------------------------------------
// common help for cli leaf functions

type help struct {
	action, description string
}

var cr_help = []help{
	{"<cr>", "perform the function"},
}

var general_help = []help{
	{"?", "display command help - Eg. ?, show ?, s?"},
	{"<up>", "go backwards in command history"},
	{"<dn>", "go forwards in command history"},
	{"<tab>", "auto complete commands"},
	{"* note", "commands can be incomplete - Eg. sh = sho = show"},
}

var history_help = []help{
	{"<cr>", "display all history"},
	{"<index>", "recall history entry <index>"},
}

//-----------------------------------------------------------------------------

const inv_arg = "invalid argument\n"

// Convert a number string to an integer.
func IntArg(ui UI, arg string, limits [2]int, base int) (int, error) {
	// convert the integer
	x, err := strconv.ParseInt(arg, base, 64)
	if err != nil {
		ui.Put(inv_arg)
		return 0, err
	}
	// check the limits
	val := int(x)
	if val < limits[0] || val > limits[1] {
		ui.Put(inv_arg)
		return 0, errors.New("out of range")
	}
	return val, nil
}

//-----------------------------------------------------------------------------

// Return a string for a list of columns.
// Each element in clist is [col0_str, col1_str, col2_str, ...]
// csize is a list of column width minimums.
func DisplayCols(clist [][]string, csize []int) string {
	// how many rows?
	nrows := len(clist)
	if nrows == 0 {
		return ""
	}
	// how many columns?
	ncols := len(clist[0])
	// make sure we have a well formed csize
	if csize == nil {
		csize = make([]int, ncols)
	} else {
		if len(csize) != ncols {
			panic("len(csize) != ncols")
		}
	}
	// check the column sizes are consistent
	for i := range clist {
		if len(clist[i]) != ncols {
			panic("mismatched number of columns")
		}
	}
	// additional column margin
	cmargin := 1
	// go through the strings and bump up csize widths if required
	for _, l := range clist {
		for i := 0; i < ncols; i++ {
			if csize[i] <= len(l[i]) {
				csize[i] = len(l[i]) + cmargin
			}
		}
	}
	// build the row format string
	fs_col := make([]string, ncols)
	for i, n := range csize {
		fs_col[i] = fmt.Sprintf("%%-%ds", n)
	}
	fs_row := strings.Join(fs_col, "")
	// generate the row strings
	row := make([]string, nrows)
	for i, l := range clist {
		// convert []string to []interface{}
		x := make([]interface{}, len(l))
		for j, v := range l {
			x[j] = v
		}
		row[i] = fmt.Sprintf(fs_row, x...)
	}
	// return rows and columns
	return strings.Join(row, "\n")
}

//-----------------------------------------------------------------------------
