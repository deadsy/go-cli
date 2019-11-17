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

// Help is a parameter help element.
type Help struct {
	Parm  string // parameter
	Descr string // description
}

// USER is an interface for low-level UI operations.
// A user provide object with this interface is passed to each leaf function.
type USER interface {
	Put(s string)
}

// MenuItem has 3 forms:
// {name string, submenu Menu, description string}: reference to submenu
// {name string, leaf func}: leaf command with generic <cr> help
// {name string, leaf func, help []Help}: leaf command with specific argument help
type MenuItem []interface{}

// Menu is a set of menu items.
type Menu []MenuItem

// Leaf is a leaf function within menu hierarchy.
type Leaf struct {
	Descr string               // description
	F     func(*CLI, []string) // leaf function
}

//-----------------------------------------------------------------------------
// common help for cli leaf functions

var crHelp = []Help{
	{"<cr>", "perform the function"},
}

var generalHelp = []Help{
	{"?", "display command help - Eg. ?, show ?, s?"},
	{"<up>", "go backwards in command history"},
	{"<dn>", "go forwards in command history"},
	{"<tab>", "auto complete commands"},
	{"* note", "commands can be incomplete - Eg. sh = sho = show"},
}

// HistoryHelp is help for the history command.
var HistoryHelp = []Help{
	{"<cr>", "display all history"},
	{"<index>", "recall history entry <index>"},
}

//-----------------------------------------------------------------------------
// argument processing

// IntArg converts a number string to an integer.
func IntArg(arg string, limits [2]int, base int) (int, error) {
	// convert the integer
	x, err := strconv.ParseInt(arg, base, 64)
	if err != nil {
		return 0, errors.New("invalid argument")
	}
	// check the limits
	val := int(x)
	if val < limits[0] || val > limits[1] {
		return 0, errors.New("invalid argument, out of range")
	}
	return val, nil
}

// UintArg converts a number string to an unsigned integer.
func UintArg(arg string, limits [2]uint, base int) (uint, error) {
	// convert the integer
	x, err := strconv.ParseUint(arg, base, 64)
	if err != nil {
		return 0, errors.New("invalid argument")
	}
	// check the limits
	val := uint(x)
	if val < limits[0] || val > limits[1] {
		return 0, errors.New("invalid argument, out of range")
	}
	return val, nil
}

// CheckArgc returns an error if the argument count is not in the valid set.
func CheckArgc(args []string, valid []int) error {
	argc := len(args)
	for i := range valid {
		if argc == valid[i] {
			return nil
		}
	}
	return errors.New("bad number of arguments")
}

//-----------------------------------------------------------------------------

// TableString returns a string for a table of row by column strings.
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
	fmtCol := make([]string, ncols)
	for i, n := range csize {
		fmtCol[i] = fmt.Sprintf("%%-%ds", n)
	}
	fmtRow := strings.Join(fmtCol, "")
	// generate the row strings
	row := make([]string, nrows)
	for i, l := range rows {
		// convert []string to []interface{}
		x := make([]interface{}, len(l))
		for j, v := range l {
			x[j] = v
		}
		row[i] = fmt.Sprintf(fmtRow, x...)
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
func splitIndex(s string) [][2]int {
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
func menuNames(menu Menu) []string {
	s := make([]string, len(menu))
	for i := range menu {
		s[i] = menu[i][0].(string)
	}
	return s
}

//-----------------------------------------------------------------------------

// Display a parse error string.
func (c *CLI) displayError(msg string, cmds []string, idx int) {
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
func (c *CLI) displayFunctionHelp(help []Help) {
	s := make([][]string, len(help))
	for i := range s {
		pStr := help[i].Parm
		var dStr string
		if len(pStr) != 0 {
			dStr = fmt.Sprintf(": %s", help[i].Descr)
		} else {
			dStr = fmt.Sprintf("  %s", help[i].Descr)
		}
		s[i] = []string{"   ", pStr, dStr}
	}
	c.Put(TableString(s, []int{0, 16, 0}, 1) + "\n")
}

// display help results for a command at a menu level
func (c *CLI) commandHelp(cmd string, menu Menu) {
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
func (c *CLI) functionHelp(item MenuItem) {
	var help []Help
	if len(item) == 3 {
		help = item[2].([]Help)
	} else {
		help = crHelp
	}
	c.displayFunctionHelp(help)
}

// Return a slice of line completion strings for the command line.
func (c *CLI) completionCallback(cmdLine string) []string {
	line := ""
	// split the command line into a list of command indices
	cmdIndices := splitIndex(cmdLine)
	// trace each command through the menu tree
	menu := c.root
	for _, index := range cmdIndices {
		cmd := cmdLine[index[0]:index[1]]
		line = cmdLine[:index[1]]
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
				return completions(line, cmd, menuNames(matches), len(cmdLine))
			}
			// we have the whole command - is this a submenu or leaf?
			if submenu, ok := item[1].(Menu); ok {
				// submenu: switch to the submenu and continue parsing
				menu = submenu
				continue
			} else {
				// leaf function: no completions to offer
				return nil
			}
		} else {
			// Multiple matches at this level. Return the matches.
			return completions(line, cmd, menuNames(matches), len(cmdLine))
		}
	}
	// We've made it here without returning a completion list.
	// The prior set of tokens have all matched single submenu items.
	// The completions are all of the items at the current menu level.
	return completions(line, "", menuNames(menu), len(cmdLine))
}

// Parse and process the current command line.
// Return a string for the new command line.
// The return string is generally empty, but may be non-empty for command history.
func (c *CLI) parseCmdline(line string) string {
	// scan the command line into a list of tokens
	cmdList := make([]string, 0, 8)
	for _, s := range strings.Split(line, " ") {
		if len(s) != 0 {
			cmdList = append(cmdList, s)
		}
	}
	// if there are no commands, print a new empty prompt
	if len(cmdList) == 0 {
		return ""
	}
	// trace each command through the menu tree
	menu := c.root
	for idx, cmd := range cmdList {
		// A trailing '?' means the user wants help for this command
		if cmd[len(cmd)-1] == '?' {
			// strip off the '?'
			cmd = cmd[:len(cmd)-1]
			c.commandHelp(cmd, menu)
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
			c.displayError("unknown command", cmdList, idx)
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
				args := cmdList[idx+1:]
				if len(args) != 0 {
					lastArg := args[len(args)-1]
					if lastArg[len(lastArg)-1] == '?' {
						c.functionHelp(item)
						// strip off the '?', repeat the command
						return line[:len(line)-1]
					}
				}
				// call the leaf function
				leaf := item[1].(Leaf).F
				leaf(c, args)
				// post leaf function actions
				if c.nextLine != "" {
					s := c.nextLine
					c.nextLine = ""
					return s
				}
				// add the command to history
				c.ln.HistoryAdd(strings.TrimSpace(line))
				// return to an empty line
				return ""
			}
		} else {
			// multiple matches - ambiguous command
			c.displayError("ambiguous command", cmdList, idx)
			return ""
		}
	}
	// reached the end of the command list with no errors and no leaf function.
	c.Put("additional input needed\n")
	return line
}

//-----------------------------------------------------------------------------

// CLI stores the CLI state.
type CLI struct {
	User        USER       // user provided object
	ln          *Linenoise // line editing object
	root        Menu       // root of menu structure
	currentLine string     // current command line
	nextLine    string     // next line set by a leaf function
	prompt      string     // cli prompt string
	running     bool       // is the cli running?
}

// NewCLI returns a new CLI object.
func NewCLI(user USER) *CLI {
	c := CLI{}
	c.User = user
	c.ln = NewLineNoise()
	c.ln.SetCompletionCallback(c.completionCallback)
	c.ln.SetHotkey('?')
	c.prompt = "> "
	c.running = true
	return &c
}

// SetRoot sets the menu root.
func (c *CLI) SetRoot(root []MenuItem) {
	c.root = root
}

// SetPrompt sets the command prompt.
func (c *CLI) SetPrompt(prompt string) {
	c.prompt = prompt
}

// SetLine sets the next command line.
func (c *CLI) SetLine(line string) {
	c.nextLine = line
}

// Loop is a passthrough to the wait for hotkey Loop().
func (c *CLI) Loop(fn func() bool, exitKey rune) bool {
	return c.ln.Loop(fn, exitKey)
}

// Put is a passthrough to the user provided Put().
func (c *CLI) Put(s string) {
	c.User.Put(s)
}

// GeneralHelp displays general help.
func (c *CLI) GeneralHelp() {
	c.displayFunctionHelp(generalHelp)
}

// HistoryLoad loads command history from a file.
func (c *CLI) HistoryLoad(path string) {
	c.ln.HistoryLoad(path)
}

// HistorySave saves command history to a file.
func (c *CLI) HistorySave(path string) {
	c.ln.HistorySave(path)
}

// DisplayHistory displays the command history.
func (c *CLI) DisplayHistory(args []string) string {
	// get the history
	h := c.ln.historyList()
	n := len(h)
	if len(args) == 1 {
		// retrieve a specific history entry
		idx, err := IntArg(args[0], [2]int{0, n - 1}, 10)
		if err != nil {
			c.User.Put(fmt.Sprintf("%s\n", err))
			return ""
		}
		// Return the next line buffer.
		// Note: linenoise wants to add the line buffer as the zero-th history entry.
		// It can only do this if it's unique- and this isn't because it's a prior
		// history entry. Make it unique by adding a trailing whitespace. The other
		// entries have been stripped prior to being added to history.
		return h[n-idx-1] + " "
	}
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
	return ""
}

// Run gets and processes a CLI command.
func (c *CLI) Run() {
	line, err := c.ln.Read(c.prompt, c.currentLine)
	if err == nil {
		c.currentLine = c.parseCmdline(line)
	} else {
		// exit: ctrl-C/ctrl-D
		c.running = false
	}
}

// Running returns true if the CLI is running.
func (c *CLI) Running() bool {
	return c.running
}

// Exit the CLI.
func (c *CLI) Exit() {
	c.running = false
}

//-----------------------------------------------------------------------------
