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

	"github.com/mattn/go-runewidth"
)

//-----------------------------------------------------------------------------

type Help struct {
	parm, descr string
}

// Each leaf function is called with an object with this interface.
type UI interface {
	Put(s string)
}

// Menu Tree
// {name, submenu, description} - reference to submenu
// {name, leaf} - leaf command with generic <cr> help
// {name, leaf, help} - leaf command with specific argument help
// Note: The general help for a leaf function is the documentation string for the leaf function.

type Menu struct {
	name string
	item []interface{}
}

//-----------------------------------------------------------------------------
// common help for cli leaf functions

var cr_help = []Help{
	{"<cr>", "perform the function"},
}

var general_help = []Help{
	{"?", "display command help - Eg. ?, show ?, s?"},
	{"<up>", "go backwards in command history"},
	{"<dn>", "go forwards in command history"},
	{"<tab>", "auto complete commands"},
	{"* note", "commands can be incomplete - Eg. sh = sho = show"},
}

var history_help = []Help{
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

// Return a string that repeats the rune n times.
func repeat(r rune, n int) string {
	x := make([]rune, n)
	for i := range x {
		x[i] = r
	}
	return string(x)
}

//-----------------------------------------------------------------------------

// split a string on whitespace and return the substring indices
func split_index(s string) [][2]int {
	// start and end with whitespace
	ws := true
	s += " "
	indices := make([][2]int, 0, 10)
	var start int
	for i, c := range s {
		if !ws && c == ' ' {
			// non-whitespace to whitespace
			ws = true
			indices = append(indices, [2]int{start, i})
		} else if ws && c != ' ' {
			// whitespace to non-whitespace
			start = i
			ws = false
		}
	}
	return indices
}

//-----------------------------------------------------------------------------

// return the list of line completions
func completions(line, cmd string, names []string, minlen int) []string {
	if cmd == "" && line != "" {
		line += " "
	}
	lines := make([]string, 0, len(names))
	for i := range lines {
		lines[i] = fmt.Sprintf("%s%s", line, names[i][len(cmd):])
		// Pad the lines to a minimum length.
		// We don't want the cursor to move about unecessarily.
		pad := minlen - len(lines[i])
		if pad > 0 {
			lines[i] += repeat(' ', pad)
		}
	}
	return lines
}

//-----------------------------------------------------------------------------

type CLI struct {
	ui      UI
	ln      *linenoise
	poll    func()
	root    []Menu
	prompt  string
	running bool
}

func NewCLI() *CLI {
	cli := CLI{}
	return &cli
}

// set the menu root
func (cli *CLI) SetRoot(root []Menu) {
	cli.root = root
}

// set the command prompt
func (cli *CLI) SetPrompt(prompt string) {
	cli.prompt = prompt
}

// set the external polling function
func (cli *CLI) set_poll(poll func()) {
	cli.poll = poll
}

// display a parse error string
func (cli *CLI) display_error(msg string, cmds []string, idx int) {
	marker := make([]string, len(cmds))
	for i := range cmds {
		n := runewidth.StringWidth(cmds[i])
		if i == idx {
			marker[i] = repeat('^', n)
		} else {
			marker[i] = repeat(' ', n)
		}
	}
	s := strings.Join([]string{msg, strings.Join(cmds, " "), strings.Join(marker, " ")}, "\n")
	cli.ui.Put(s + "\n")
}

// display function help
func (cli *CLI) display_function_help(help []Help) {
	s := make([][]string, len(help))
	for i := range s {
		p_str := help[i].parm
		var d_str string
		if len(p_str) != 0 {
			d_str = fmt.Sprintf(": %s", help[i].descr)
		} else {
			d_str = fmt.Sprintf("  %s", help[i].descr)
		}
		s[i] = []string{"   ", p_str, d_str}
	}
	cli.ui.Put(DisplayCols(s, []int{0, 16, 0}) + "\n")
}

// display help results for a command at a menu level
func (cli *CLI) command_help(cmd string, menu []Menu) {
	s := make([][]string, len(menu))
	for i, v := range menu {
		if strings.HasPrefix(v.name, cmd) {
			var descr string
			if _, ok := v.item[1].([]Menu); ok {
				// submenu: the next string is the help
				descr = v.item[2].(string)
			} else {
				// command: docstring is the help
				descr = "TODO docstring"
				//descr = item[1].__doc__
			}
			s[i] = []string{"  ", v.name, fmt.Sprintf(": %s", descr)}
		}
	}
	cli.ui.Put(DisplayCols(s, []int{0, 16, 0}) + "\n")
}

// display help for a leaf function
func (cli *CLI) function_help(menu Menu) {
	var help []Help
	if len(menu.item) == 2 {
		help = menu.item[1].([]Help)
	} else {
		help = cr_help
	}
	cli.display_function_help(help)
}

// display general help
func (cli *CLI) general_help() {
	cli.display_function_help(general_help)
}

// display the command history
func (cli *CLI) display_history(args []string) string {
	// get the history
	h := cli.ln.history_list()
	n := len(h)
	if len(args) == 1 {
		// retrieve a specific history entry
		idx, err := IntArg(cli.ui, args[0], [2]int{0, n - 1}, 10)
		if err != nil {
			return ""
		}
		// Return the next line buffer.
		// Note: linenoise wants to add the line buffer as the zero-th history entry.
		// It can only do this if it's unique- and this isn't because it's a prior
		// history entry. Make it unique by adding a trailing whitespace. The other
		// entries have been stripped prior to being added to history.
		return h[n-idx-1] + " "
	} else {
		// display all history
		if n > 0 {
			s := make([]string, n)
			for i := range s {
				s[i] = fmt.Sprintf("%-3d: %s", n-i-1, h[i])
			}
			cli.ui.Put(strings.Join(s, "\n") + "\n")
		} else {
			cli.ui.Put("no history\n")
		}
	}
	return ""
}

/*

// return a tuple of line completions for the command line
func (cli *CLI) completion_callback(self, cmd_line):
    line = ''
    # split the command line into a list of command indices
    cmd_list = split_index(cmd_line)
    # trace each command through the menu tree
    menu = self.root
    for (start, end) in cmd_list:
      cmd = cmd_line[start:end]
      line = cmd_line[:end]
      # How many items does this token match at this level of the menu?
      matches = [x for x in menu if x[0].startswith(cmd)]
      if len(matches) == 0:
        # no matches, no completions
        return None
      elif len(matches) == 1:
        item = matches[0]
        if len(cmd) < len(item[0]):
          # it's an unambiguous single match, but we still complete it
          return self.completions(line, len(cmd_line), cmd, [item[0],])
        else:
          # we have the whole command - is this a submenu or leaf?
          if isinstance(item[1], tuple):
            # submenu: switch to the submenu and continue parsing
            menu = item[1]
            continue
          else:
            # leaf function: no completions to offer
            return None
      else:
        # Multiple matches at this level. Return the matches.
        return self.completions(line, len(cmd_line), cmd, [x[0] for x in matches])
    # We've made it here without returning a completion list.
    # The prior set of tokens have all matched single submenu items.
    # The completions are all of the items at the current menu level.
    return self.completions(line, len(cmd_line), '', [x[0] for x in menu])

// Parse and process the current command line.
// Return a string for the new command line.
// This is generally empty, but may be non-empty if the user needs to edit a pre-entered command.
func (cli *CLI) parse_cmdline(line string) string {

    # scan the command line into a list of tokens
    cmd_list = [x for x in line.split(' ') if x != '']
    # if there are no commands, print a new empty prompt
    if len(cmd_list) == 0:
      return ''
    # trace each command through the menu tree
    menu = self.root
    for (idx, cmd) in enumerate(cmd_list):
      # A trailing '?' means the user wants help for this command
      if cmd[-1] == '?':
        # strip off the '?'
        cmd = cmd[:-1]
        self.command_help(cmd, menu)
        # strip off the '?' and recycle the command
        return line[:-1]
      # try to match the cmd with a unique menu item
      matches = []
      for item in menu:
        if item[0] == cmd:
          # accept an exact match
          matches = [item]
          break
        if item[0].startswith(cmd):
          matches.append(item)
      if len(matches) == 0:
        # no matches - unknown command
        self.display_error('unknown command', cmd_list, idx)
        # add it to history in case the user wants to edit this junk
        self.ln.history_add(line.strip())
        # go back to an empty prompt
        return ''
      if len(matches) == 1:
        # one match - submenu/leaf
        item = matches[0]
        if isinstance(item[1], tuple):
          # this is a submenu
          # switch to the submenu and continue parsing
          menu = item[1]
          continue
        else:
          # this is a leaf function - get the arguments
          args = cmd_list[idx:]
          del args[0]
          if len(args) != 0:
            if args[-1][-1] == '?':
              self.function_help(item)
              # strip off the '?', repeat the command
              return line[:-1]
          # call the leaf function
          rc = item[1](self.ui, args)
          # post leaf function actions
          if rc is not None:
            # currently only history retrieval returns not None
            # the return code is the next line buffer
            return rc
          else:
            # add the command to history
            self.ln.history_add(line.strip())
            # return to an empty line
            return ''
      else:
        # multiple matches - ambiguous command
        self.display_error('ambiguous command', cmd_list, idx)
        return ''
    # reached the end of the command list with no errors and no leaf function.
    self.ui.put('additional input needed\n')
    return line
}

// get and process cli commands in a loop
func (cli *CLI) run() {
    line = ''
    while self.running:
      line = self.ln.read(self.prompt, line)
      if line is not None:
        line = self.parse_cmdline(line)
      else:
        # exit: ctrl-C/ctrl-D
        self.running = False
    self.ln.history_save('history.txt')
}

*/

// exit the cli
func (cli *CLI) exit() {
	cli.running = false
}

//-----------------------------------------------------------------------------
