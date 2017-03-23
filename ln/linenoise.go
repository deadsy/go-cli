//-----------------------------------------------------------------------------
/*

linenoise for golang

See: https://github.com/deadsy/go_linenoise

Based on: http://github.com/antirez/linenoise

*/
//-----------------------------------------------------------------------------

package ln

import (
	"fmt"

	"github.com/mattn/go-isatty"
)

//-----------------------------------------------------------------------------

type linestate struct {
	ifd         uintptr    // stdin file descriptor
	ofd         uintptr    // stdout file descriptor
	prompt      string     // prompt string
	ts          *linenoise // terminal state
	history_idx int        // history index we are currently editing, 0 is the LAST entry
	buf         []rune     // line buffer
	cols        int        // number of columns in terminal
	pos         int        // current cursor position within line buffer
	oldpos      int        // previous refresh cursor position (multiline)
	maxrows     int        // maximum num of rows used so far (multiline)
}

//-----------------------------------------------------------------------------

type linenoise struct {
	history        []string //list of history strings
	history_maxlen int      // maximum number of history entries
	rawmode        bool     // are we in raw mode?
	mlmode         bool     // are we in multiline mode?

	//self.orig_termios = None        // saved termios attributes
	//self.completion_callback = None // callback function for tab completion
	//self.hints_callback = None      // callback function for hints
	//self.hotkey = None              // character for hotkey
}

func NewLineNoise() *linenoise {
	l := linenoise{}
	l.history_maxlen = 32
	return &l
}

// Enable raw mode
func (l *linenoise) enable_rawmode(fd uintptr) error {
	if !isatty.IsTerminal(fd) {
		return fmt.Errorf("fd %d is not a tty", fd)
	}

	return nil
}

// Disable raw mode
func (l *linenoise) disable_rawmode(fd uintptr) {
}

// Restore STDIN to the orignal mode
func (l *linenoise) atexit() {
}

//-----------------------------------------------------------------------------
