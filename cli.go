//-----------------------------------------------------------------------------
/*

Command Line Interface

Implements a CLI with:

* hierarchical menus
* command tab completion
* command history
* context sensitive help
* command editing

*/
//-----------------------------------------------------------------------------

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

//-----------------------------------------------------------------------------

// Help
type Help struct {
	Parm  string // parameter
	Descr string // description
}

// USER: This is a user object made available to each leaf function.
type USER interface {
	Put(s string)
}

// Menu Item: 3 forms
// {name string, submenu Menu, description string}: reference to submenu
// {name string, leaf func}: leaf command with generic <cr> help
// {name string, leaf func, help []Help}: leaf command with specific argument help
type MenuItem []interface{}

// Menu: a set of menu items
type Menu []MenuItem

// Leaf: leaf function within menu hierarchy.
type Leaf struct {
	Descr string               // description
	F     func(*CLI, []string) // leaf function
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

var HistoryHelp = []Help{
	{"<cr>", "display all history"},
	{"<index>", "recall history entry <index>"},
}

//-----------------------------------------------------------------------------

const inv_arg = "invalid argument\n"

// Convert a number string to an integer.
func IntArg(user USER, arg string, limits [2]int, base int) (int, error) {
	// convert the integer
	x, err := strconv.ParseInt(arg, base, 64)
	if err != nil {
		user.Put(inv_arg)
		return 0, err
	}
	// check the limits
	val := int(x)
	if val < limits[0] || val > limits[1] {
		user.Put(inv_arg)
		return 0, errors.New("out of range")
	}
	return val, nil
}

//-----------------------------------------------------------------------------

// Return a string for a table of row by column strings
// Each column string will be left justified and aligned.
func TableString(
	rows [][]string, // table rows [[col0, col1, col2...,colN]...]
	csize []int, // minimum column widths
	cmargin int, // column to column margin
) string {
	// how many rows?
	nrows := len(rows)
	if nrows == 0 {
		return ""
	}
	// how many columns?
	ncols := len(rows[0])
	// make sure we have a well formed csize
	if csize == nil {
		csize = make([]int, ncols)
	} else {
		if len(csize) != ncols {
			panic("len(csize) != ncols")
		}
	}
	// check that the number of columns for each row is consistent
	for i := range rows {
		if len(rows[i]) != ncols {
			panic(fmt.Sprintf("ncols row%d != ncols row0", i))
		}
	}
	// go through the strings and bump up csize widths if required
	for i := 0; i < nrows; i++ {
		for j := 0; j < ncols; j++ {
			width := runewidth.StringWidth(rows[i][j])
			if (width + cmargin) >= csize[j] {
				csize[j] = width + cmargin
			}
		}
	}
	// build the row format string
	fmt_col := make([]string, ncols)
	for i, n := range csize {
		fmt_col[i] = fmt.Sprintf("%%-%ds", n)
	}
	fmt_row := strings.Join(fmt_col, "")
	// generate the row strings
	row := make([]string, nrows)
	for i, l := range rows {
		// convert []string to []interface{}
		x := make([]interface{}, len(l))
		for j, v := range l {
			x[j] = v
		}
		row[i] = fmt.Sprintf(fmt_row, x...)
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

// Split a string on whitespace and return the substring indices.
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

// Return the list of line completions.
func completions(line, cmd string, names []string, minlen int) []string {
	// if we are completing a complete word then add a space
	if cmd == "" && line != "" {
		line += " "
	}
	lines := make([]string, len(names))
	for i := range lines {
		lines[i] = fmt.Sprintf("%s%s", line, names[i][len(cmd):])
		// Pad the lines to a minimum length.
		// We don't want the cursor to move about unecessarily.
		pad := minlen - runewidth.StringWidth(lines[i])
		if pad > 0 {
			lines[i] += repeat(' ', pad)
		}
	}
	return lines
}

// Return a list of menu names.
func menu_names(menu Menu) []string {
	s := make([]string, len(menu))
	for i := range menu {
		s[i] = menu[i][0].(string)
	}
	return s
}

//-----------------------------------------------------------------------------

// Display a parse error string.
func (c *CLI) display_error(msg string, cmds []string, idx int) {
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
	c.Put(s + "\n")
}

// display function help
func (c *CLI) display_function_help(help []Help) {
	s := make([][]string, len(help))
	for i := range s {
		p_str := help[i].Parm
		var d_str string
		if len(p_str) != 0 {
			d_str = fmt.Sprintf(": %s", help[i].Descr)
		} else {
			d_str = fmt.Sprintf("  %s", help[i].Descr)
		}
		s[i] = []string{"   ", p_str, d_str}
	}
	c.Put(TableString(s, []int{0, 16, 0}, 1) + "\n")
}

// display help results for a command at a menu level
func (c *CLI) command_help(cmd string, menu Menu) {
	s := make([][]string, 0, len(menu))
	for _, item := range menu {
		name := item[0].(string)
		if strings.HasPrefix(name, cmd) {
			var descr string
			switch item[1].(type) {
			case Menu:
				// submenu: the next string is the help
				descr = item[2].(string)
			case Leaf:
				// command: use leaf function description
				descr = item[1].(Leaf).Descr
			default:
				panic("unknown type")
			}
			s = append(s, []string{"  ", name, fmt.Sprintf(": %s", descr)})
		}
	}
	c.Put(TableString(s, []int{0, 16, 0}, 1) + "\n")
}

// display help for a leaf function
func (c *CLI) function_help(item MenuItem) {
	var help []Help
	if len(item) == 3 {
		help = item[2].([]Help)
	} else {
		help = cr_help
	}
	c.display_function_help(help)
}

// Return a slice of line completion strings for the command line.
func (c *CLI) completion_callback(cmd_line string) []string {
	line := ""
	// split the command line into a list of command indices
	cmd_indices := split_index(cmd_line)
	// trace each command through the menu tree
	menu := c.root
	for _, index := range cmd_indices {
		cmd := cmd_line[index[0]:index[1]]
		line = cmd_line[:index[1]]
		// How many items does this token match at this level of the menu?
		matches := make([]MenuItem, 0, len(menu))
		for _, item := range menu {
			if strings.HasPrefix(item[0].(string), cmd) {
				matches = append(matches, item)
			}
		}
		if len(matches) == 0 {
			// no matches, no completions
			return nil
		} else if len(matches) == 1 {
			item := matches[0]
			if len(cmd) < len(item[0].(string)) {
				// it's an unambiguous single match, but we still complete it
				return completions(line, cmd, menu_names(matches), len(cmd_line))
			} else {
				// we have the whole command - is this a submenu or leaf?
				if submenu, ok := item[1].(Menu); ok {
					// submenu: switch to the submenu and continue parsing
					menu = submenu
					continue
				} else {
					// leaf function: no completions to offer
					return nil
				}
			}
		} else {
			// Multiple matches at this level. Return the matches.
			return completions(line, cmd, menu_names(matches), len(cmd_line))
		}
	}
	// We've made it here without returning a completion list.
	// The prior set of tokens have all matched single submenu items.
	// The completions are all of the items at the current menu level.
	return completions(line, "", menu_names(menu), len(cmd_line))
}

// Parse and process the current command line.
// Return a string for the new command line.
// The return string is generally empty, but may be non-empty for command history.
func (c *CLI) parse_cmdline(line string) string {
	// scan the command line into a list of tokens
	cmd_list := make([]string, 0, 8)
	for _, s := range strings.Split(line, " ") {
		if len(s) != 0 {
			cmd_list = append(cmd_list, s)
		}
	}
	// if there are no commands, print a new empty prompt
	if len(cmd_list) == 0 {
		return ""
	}
	// trace each command through the menu tree
	menu := c.root
	for idx, cmd := range cmd_list {
		// A trailing '?' means the user wants help for this command
		if cmd[len(cmd)-1] == '?' {
			// strip off the '?'
			cmd = cmd[:len(cmd)-1]
			c.command_help(cmd, menu)
			// strip off the '?' and recycle the command
			return line[:len(line)-1]
		}
		// try to match the cmd with a unique menu item
		matches := make([]MenuItem, 0, len(menu))
		for _, item := range menu {
			if item[0].(string) == cmd {
				// accept an exact match
				matches = []MenuItem{item}
				break
			}
			if strings.HasPrefix(item[0].(string), cmd) {
				matches = append(matches, item)
			}
		}
		if len(matches) == 0 {
			// no matches - unknown command
			c.display_error("unknown command", cmd_list, idx)
			// add it to history in case the user wants to edit this junk
			c.ln.HistoryAdd(strings.TrimSpace(line))
			// go back to an empty prompt
			return ""
		}
		if len(matches) == 1 {
			// one match - submenu/leaf
			item := matches[0]
			if submenu, ok := item[1].(Menu); ok {
				// submenu, switch to the submenu and continue parsing
				menu = submenu
				continue
			} else {
				// leaf function - get the arguments
				args := cmd_list[idx+1:]
				if len(args) != 0 {
					last_arg := args[len(args)-1]
					if last_arg[len(last_arg)-1] == '?' {
						c.function_help(item)
						// strip off the '?', repeat the command
						return line[:len(line)-1]
					}
				}
				// call the leaf function
				leaf := item[1].(Leaf).F
				leaf(c, args)
				// post leaf function actions
				if c.next_line != "" {
					s := c.next_line
					c.next_line = ""
					return s
				} else {
					// add the command to history
					c.ln.HistoryAdd(strings.TrimSpace(line))
					// return to an empty line
					return ""
				}
			}
		} else {
			// multiple matches - ambiguous command
			c.display_error("ambiguous command", cmd_list, idx)
			return ""
		}
	}
	// reached the end of the command list with no errors and no leaf function.
	c.Put("additional input needed\n")
	return line
}

//-----------------------------------------------------------------------------

type CLI struct {
	User      USER       // user provided object
	ln        *linenoise // line editing object
	root      Menu       // root of menu structure
	next_line string     // next line set by a leaf function
	prompt    string     // cli prompt string
	running   bool       // is the cli running?
}

func NewCLI(user USER) *CLI {
	c := CLI{}
	c.User = user
	c.ln = NewLineNoise()
	c.ln.SetCompletionCallback(c.completion_callback)
	c.ln.SetHotkey('?')
	c.prompt = "> "
	c.running = true
	return &c
}

// set the menu root
func (c *CLI) SetRoot(root []MenuItem) {
	c.root = root
}

// set the command prompt
func (c *CLI) SetPrompt(prompt string) {
	c.prompt = prompt
}

// Set the next command line.
func (c *CLI) SetLine(line string) {
	c.next_line = line
}

// Passthrough to the wait for hotkey Loop().
func (c *CLI) Loop(fn func() bool, exit_key rune) bool {
	return c.ln.Loop(fn, exit_key)
}

// Passthrough to the user provided Put().
func (c *CLI) Put(s string) {
	c.User.Put(s)
}

// Display general help.
func (c *CLI) GeneralHelp() {
	c.display_function_help(general_help)
}

// Load command history from a file.
func (c *CLI) HistoryLoad(path string) {
	c.ln.HistoryLoad(path)
}

// Save command history to a file.
func (c *CLI) HistorySave(path string) {
	c.ln.HistorySave(path)
}

// Display the command history.
func (c *CLI) DisplayHistory(args []string) string {
	// get the history
	h := c.ln.history_list()
	n := len(h)
	if len(args) == 1 {
		// retrieve a specific history entry
		idx, err := IntArg(c.User, args[0], [2]int{0, n - 1}, 10)
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
			c.Put(strings.Join(s, "\n") + "\n")
		} else {
			c.Put("no history\n")
		}
	}
	return ""
}

// Get and process a CLI command.
func (c *CLI) Run() {
	line := ""
	line, err := c.ln.Read(c.prompt, line)
	if err == nil {
		line = c.parse_cmdline(line)
	} else {
		// exit: ctrl-C/ctrl-D
		c.running = false
	}
}

func (c *CLI) Running() bool {
	return c.running
}

// Exit the CLI.
func (c *CLI) Exit() {
	c.running = false
}

//-----------------------------------------------------------------------------
