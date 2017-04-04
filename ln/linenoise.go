//-----------------------------------------------------------------------------
/*

linenoise for golang

See: https://github.com/deadsy/go_linenoise

Based on: http://github.com/antirez/linenoise

*/
//-----------------------------------------------------------------------------

package ln

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"
	"unicode"

	"github.com/creack/termios/raw"
	"github.com/mattn/go-isatty"
)

//-----------------------------------------------------------------------------

// Keycodes
const KEYCODE_NULL = 0
const KEYCODE_CTRL_A = 1
const KEYCODE_CTRL_B = 2
const KEYCODE_CTRL_C = 3
const KEYCODE_CTRL_D = 4
const KEYCODE_CTRL_E = 5
const KEYCODE_CTRL_F = 6
const KEYCODE_CTRL_H = 8
const KEYCODE_TAB = 9
const KEYCODE_LF = 10
const KEYCODE_CTRL_K = 11
const KEYCODE_CTRL_L = 12
const KEYCODE_CR = 13
const KEYCODE_CTRL_N = 14
const KEYCODE_CTRL_P = 16
const KEYCODE_CTRL_T = 20
const KEYCODE_CTRL_U = 21
const KEYCODE_CTRL_W = 23
const KEYCODE_ESC = 27
const KEYCODE_BS = 127

//-----------------------------------------------------------------------------

// Set a tty terminal to raw mode.
func set_rawmode(fd int) (*raw.Termios, error) {
	// make sure this is a tty
	if !isatty.IsTerminal(uintptr(fd)) {
		return nil, fmt.Errorf("fd %d is not a tty", fd)
	}
	// get the terminal IO mode
	original_mode, err := raw.TcGetAttr(uintptr(fd))
	if err != nil {
		return nil, err
	}
	// modify the original mode
	new_mode := *original_mode
	new_mode.Iflag &^= (syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON)
	new_mode.Oflag &^= syscall.OPOST
	new_mode.Lflag &^= (syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN)
	new_mode.Cflag &^= (syscall.CSIZE | syscall.PARENB)
	new_mode.Cflag |= syscall.CS8
	new_mode.Cc[syscall.VMIN] = 1
	new_mode.Cc[syscall.VTIME] = 0
	err = raw.TcSetAttr(uintptr(fd), &new_mode)
	if err != nil {
		return nil, err
	}
	return original_mode, nil
}

// Restore the terminal mode.
func restore_mode(fd int, mode *raw.Termios) error {
	return raw.TcSetAttr(uintptr(fd), mode)
}

//-----------------------------------------------------------------------------

func get_rune(ch chan rune) {
	reader := bufio.NewReader(os.Stdin)
	for {
		r, size, err := reader.ReadRune()
		if err != nil {
			log.Printf("%s\n", err)
			close(ch)
			return
		}
		if size == 1 && r == unicode.ReplacementChar {
			log.Printf("invalid unicode")
		} else {
			ch <- r
		}
	}
}

//-----------------------------------------------------------------------------

var STDIN = syscall.Stdin
var STDOUT = syscall.Stdout
var STDERR = syscall.Stderr

/*

var TIMEOUT_20ms = syscall.Timeval{0, 20 * 1000}
var TIMEOUT_10ms = syscall.Timeval{0, 10 * 1000}

// read a single rune from a file (with timeout)
// timeout >= 0 : wait for timeout seconds
// timeout = nil : return immediately
func get_rune(fd int, timeout *syscall.Timeval) rune {
	// use select() for the timeout
	if timeout != nil {
		rd := syscall.FdSet{}
		fdset.Set(fd, &rd)
		n, err := syscall.Select(fd+1, &rd, nil, nil, timeout)
		if err != nil {
			panic(fmt.Sprintf("select error %s\n", err))
		}
		if n == 0 {
			return RUNE_NULL
		}
	}
	c := make([]byte, 1)
	n, err := syscall.Read(fd, c)
	if err != nil {
		panic(fmt.Sprintf("read error %s\n", err))
	}
	if n == 0 {
		return RUNE_NULL
	}
	return rune(c[0])
}



// If fd is not readable within the timeout period return true.
func would_block(fd int, timeout *syscall.Timeval) bool {
	rd := syscall.FdSet{}
	fdset.Set(fd, &rd)
	n, err := syscall.Select(fd+1, &rd, nil, nil, timeout)
	if err != nil {
		panic(fmt.Sprintf("select error %s\n", err))
	}
	return n == 0
}

*/

// Write a string to the file descriptor, return the number of bytes written.
func puts(fd int, s string) int {
	n, err := syscall.Write(fd, []byte(s))
	if err != nil {
		panic(fmt.Sprintf("puts error %s\n", err))
	}
	return n
}

//-----------------------------------------------------------------------------

// Use this value if we can't work out how many columns the terminal has.
const DEFAULT_COLS = 80

// Get the horizontal cursor position
func get_cursor_position(ifd, ofd int) int {

	// query the cursor location
	if puts(ofd, "\x1b[6n") != 4 {
		return -1
	}

	/*


	    # read the response: ESC [ rows ; cols R
	  # rows/cols are decimal number strings
	  buf = []
	  while len(buf) < 32:
	    c = _getc(ifd, _CHAR_TIMEOUT)
	    if c == _KEY_NULL:
	      break
	    buf.append(c)
	    if buf[-1] == 'R':
	      break
	  # parse it: esc [ number ; number R (at least 6 characters)
	  if len(buf) < 6 or buf[0] != _KEY_ESC or buf[1] != '[' or buf[-1] != 'R':
	    return -1
	  # should have 2 number fields
	  x = ''.join(buf[2:-1]).split(';')
	  if len(x) != 2:
	    return -1
	  (_, cols) = x
	  # return the cols
	  return int(cols, 10)

	*/

	return 0
}

// Get the number of columns for the terminal. Assume DEFAULT_COLS if it fails.
func get_columns(ifd, ofd int) int {
	cols := 0

	/*

	  # try using the ioctl to get the number of cols
	  try:
	    t = fcntl.ioctl(_STDOUT, termios.TIOCGWINSZ, struct.pack('HHHH', 0, 0, 0, 0))
	    (_, cols, _, _) = struct.unpack('HHHH', t)
	  except:
	    pass
	  if cols == 0:
	    # the ioctl failed - try using the terminal itself
	    start = get_cursor_position(ifd, ofd)
	    if start < 0:
	      return _DEFAULT_COLS
	    # Go to right margin and get position
	    if _puts(ofd, '\x1b[999C') != 6:
	      return _DEFAULT_COLS
	    cols = get_cursor_position(ifd, ofd)
	    if cols < 0:
	      return _DEFAULT_COLS
	    # restore the position
	    if cols > start:
	      _puts(ofd, '\x1b[%dD' % (cols - start))


	*/

	return cols
}

//-----------------------------------------------------------------------------

var unsupported = map[string]bool{
	"dumb":   true,
	"cons25": true,
	"emacs":  true,
}

// Return true if we know we don't support this terminal.
func unsupported_term() bool {
	_, ok := unsupported[os.Getenv("TERM")]
	return ok
}

//-----------------------------------------------------------------------------

type linestate struct {
	ifd, ofd    int        // stdin/stdout file descriptors
	prompt      string     // prompt string
	ts          *linenoise // terminal state
	history_idx int        // history index we are currently editing, 0 is the LAST entry
	buf         []rune     // line buffer
	cols        int        // number of columns in terminal
	pos         int        // current cursor position within line buffer
	oldpos      int        // previous refresh cursor position (multiline)
	maxrows     int        // maximum num of rows used so far (multiline)
}

func NewLineState(ifd, ofd int, prompt string, ts *linenoise) *linestate {
	ls := linestate{}
	ls.ifd = ifd
	ls.ofd = ofd
	ls.prompt = prompt
	ls.ts = ts
	ls.cols = get_columns(ifd, ofd)
	return &ls
}

// single line refresh
func (ls *linestate) refresh_singleline() {
	panic("")
}

// multiline refresh
func (ls *linestate) refresh_multiline() {
	panic("")
}

// refresh the edit line
func (ls *linestate) refresh_line() {
	if ls.ts.mlmode {
		ls.refresh_multiline()
	} else {
		ls.refresh_singleline()
	}
}

// set the line buffer to a string
func (ls *linestate) edit_set(s string) {
	if len(s) == 0 {
		return
	}
	ls.buf = []rune(s)
	ls.pos = len(ls.buf)
	ls.refresh_line()
}

//-----------------------------------------------------------------------------

type linenoise struct {
	history        []string     //list of history strings
	history_maxlen int          // maximum number of history entries
	rawmode        bool         // are we in raw mode?
	mlmode         bool         // are we in multiline mode?
	saved_mode     *raw.Termios // saved terminal mode

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
func (l *linenoise) enable_rawmode(fd int) error {
	mode, err := set_rawmode(fd)
	if err != nil {
		return err
	}
	l.rawmode = true
	l.saved_mode = mode
	return nil
}

// Disable raw mode
func (l *linenoise) disable_rawmode(fd int) error {
	if l.rawmode {
		err := restore_mode(fd, l.saved_mode)
		if err != nil {
			return err
		}
	}
	l.rawmode = false
	return nil
}

// Restore STDIN to the orignal mode
func (l *linenoise) atexit() {
}

// edit a line in raw mode
func (l *linenoise) edit(
	ifd int, // input file descriptor
	ofd int, // output file descriptor
	prompt string, // line prompt string
	s string, // initial line string
) {

	// create the line state

	ls := NewLineState(ifd, ofd, prompt, l)

	// set and output the initial line
	ls.edit_set(s)

}

// Call the provided function in a loop.
// Exit when the function returns true or when the exit key is pressed.
// Returns true when the loop function completes, false for early exit.
func (l *linenoise) Loop(fn func() bool, exit_key rune) bool {
	err := l.enable_rawmode(STDIN)
	if err != nil {
		log.Printf("enable rawmode error %s\n", err)
		return false
	}
	defer l.disable_rawmode(STDIN)
	defer os.Stdin.Close()

	ch := make(chan rune)
	go get_rune(ch)
	for {
		select {
		case r, ok := <-ch:
			if ok && r == exit_key {
				// the loop has been cancelled
				return false
			}
		default:
			if fn() {
				// the loop function has completed
				return true
			}
		}
	}
	return false
}

// Print scan codes on screen for debugging/development purposes
func (l *linenoise) PrintKeycodes() {

	fmt.Printf("Linenoise key codes debugging mode.\n")
	fmt.Printf("Press keys to see scan codes. Type 'quit' at any time to exit.\n")

	err := l.enable_rawmode(STDIN)
	if err != nil {
		log.Printf("enable rawmode error %s\n", err)
		return
	}
	defer l.disable_rawmode(STDIN)

	ch := make(chan rune)
	go get_rune(ch)

	var cmd [4]rune
	running := true

	for running {
		select {
		case r, ok := <-ch:
			if ok {
				// display the character
				var s string
				if unicode.IsPrint(r) {
					s = string(r)
				} else {
					switch r {
					case KEYCODE_CR:
						s = "\\r"
					case KEYCODE_TAB:
						s = "\\t"
					case KEYCODE_ESC:
						s = "ESC"
					case KEYCODE_LF:
						s = "\\n"
					case KEYCODE_BS:
						s = "BS"
					default:
						s = "?"
					}
				}
				fmt.Printf("'%s' 0x%x (%d)\r\n", s, int32(r), int32(r))

				// check for quit
				copy(cmd[:], cmd[1:])
				cmd[3] = r
				if string(cmd[:]) == "quit" {
					running = false
				}

			} else {
				log.Printf("get_rune() has closed the channel")
				break
			}

		}
	}

}

// Set multiline mode
func (ln *linenoise) SetMultiline(mode bool) {
	ln.mlmode = mode
}

//-----------------------------------------------------------------------------

func foo1() {
	os.Stdin.Close()
}

func (l *linenoise) Foo() {

	err := l.enable_rawmode(STDIN)
	if err != nil {
		log.Printf("enable rawmode error %s\n", err)
		return
	}
	defer foo1()
	defer l.disable_rawmode(STDIN)

	ch := make(chan rune)
	go get_rune(ch)

	time.Sleep(1 * time.Second)

}

//-----------------------------------------------------------------------------
