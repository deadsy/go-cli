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
	"io"
	"log"
	"os"
	"strings"
	"syscall"
	"unicode"

	"github.com/creack/termios/raw"
	"github.com/mattn/go-isatty"
	"github.com/mistsys/mist_go_utils/fdset"
)

//-----------------------------------------------------------------------------

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

var STDIN = syscall.Stdin
var STDOUT = syscall.Stdout
var STDERR = syscall.Stderr

var TIMEOUT_20ms = syscall.Timeval{0, 20 * 1000}
var TIMEOUT_10ms = syscall.Timeval{0, 10 * 1000}

//-----------------------------------------------------------------------------
// control the terminal mode

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
// UTF8 Decoding

const (
	UTF8_BYTE0 = iota
	UTF8_3MORE
	UTF8_2MORE
	UTF8_1MORE
)

type utf8 struct {
	state byte
	count int
	val   int32
}

// Add a byte to a utf8 decode.
// Return the rune and it's size in bytes.
func (u *utf8) add(c byte) (r rune, size int) {
	switch u.state {
	case UTF8_BYTE0:
		if c&0x80 == 0 {
			// 1 byte
			return rune(c), 1
		} else if c&0xe0 == 0xc0 {
			// 2 byte
			u.val = int32(c&0x1f) << 6
			u.count = 2
			u.state = UTF8_1MORE
			return KEYCODE_NULL, 0
		} else if c&0xf0 == 0xe0 {
			// 3 bytes
			u.val = int32(c&0x0f) << 6
			u.count = 3
			u.state = UTF8_2MORE
			return KEYCODE_NULL, 0
		} else if c&0xf8 == 0xf0 {
			// 4 bytes
			u.val = int32(c&0x07) << 6
			u.count = 4
			u.state = UTF8_3MORE
			return KEYCODE_NULL, 0
		}
	case UTF8_3MORE:
		if c&0xc0 == 0x80 {
			u.state = UTF8_2MORE
			u.val |= int32(c & 0x3f)
			u.val <<= 6
			return KEYCODE_NULL, 0
		}
	case UTF8_2MORE:
		if c&0xc0 == 0x80 {
			u.state = UTF8_1MORE
			u.val |= int32(c & 0x3f)
			u.val <<= 6
			return KEYCODE_NULL, 0
		}
	case UTF8_1MORE:
		if c&0xc0 == 0x80 {
			u.state = UTF8_BYTE0
			u.val |= int32(c & 0x3f)
			return rune(u.val), u.count
		}
	}
	// Error
	u.state = UTF8_BYTE0
	return unicode.ReplacementChar, 1
}

// read a single rune from a file descriptor (with timeout)
// timeout >= 0 : wait for timeout seconds
// timeout = nil : return immediately
func (u *utf8) get_rune(fd int, timeout *syscall.Timeval) rune {
	// use select() for the timeout
	if timeout != nil {
		rd := syscall.FdSet{}
		fdset.Set(fd, &rd)
		n, err := syscall.Select(fd+1, &rd, nil, nil, timeout)
		if err != nil {
			panic(fmt.Sprintf("select error %s\n", err))
		}
		if n == 0 {
			// nothing is readable
			return KEYCODE_NULL
		}
	}

	// Read the file descriptor
	buf := make([]byte, 1)
	_, err := syscall.Read(fd, buf)
	if err != nil {
		panic(fmt.Sprintf("read error %s\n", err))
	}

	// decode the utf8
	r, size := u.add(buf[0])
	if size == 0 {
		// incomplete utf8 code point
		return KEYCODE_NULL
	}
	if size == 1 && r == unicode.ReplacementChar {
		// utf8 decode error
		return KEYCODE_NULL
	}
	return r
}

//-----------------------------------------------------------------------------

/*

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

func (ls *linestate) String() string {
	return string(ls.buf)
}

//-----------------------------------------------------------------------------

type linenoise struct {
	history             []string              //list of history strings
	history_maxlen      int                   // maximum number of history entries
	rawmode             bool                  // are we in raw mode?
	mlmode              bool                  // are we in multiline mode?
	saved_mode          *raw.Termios          // saved terminal mode
	completion_callback func(string) []string // callback function for tab completion
	hints_callback      func(string) *Hint    // callback function for hints
	hotkey              rune                  // character for hotkey
	scanner             *bufio.Scanner        // buffered IO scanner for file reading
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

//-----------------------------------------------------------------------------

// Restore STDIN to the orignal mode
func (l *linenoise) atexit() {
}

//-----------------------------------------------------------------------------

// edit a line in raw mode
func (l *linenoise) edit(
	ifd int, // input file descriptor
	ofd int, // output file descriptor
	prompt string, // line prompt string
	s string, // initial line string
) *string {
	// create the line state
	ls := NewLineState(ifd, ofd, prompt, l)
	// set and output the initial line
	ls.edit_set(s)

	// The latest history entry is always our current buffer
	l.HistoryAdd(ls.String())

	return nil
}

//-----------------------------------------------------------------------------

// Read a line from stdin in raw mode.
func (l *linenoise) read_raw(prompt, s string) *string {

	// set rawmode for stdin
	err := l.enable_rawmode(STDIN)
	if err != nil {
		log.Printf("enable rawmode error %s\n", err)
		return nil
	}

	line := l.edit(STDIN, STDOUT, prompt, s)

	l.disable_rawmode(STDIN)

	fmt.Printf("\r\n")

	return line
}

// Read a line. Return nil on EOF/quit.
func (l *linenoise) Read(prompt, s string) *string {
	if !isatty.IsTerminal(uintptr(STDIN)) {
		// Not a tty, read from a file or pipe.
		if l.scanner == nil {
			l.scanner = bufio.NewScanner(os.Stdin)
		}
		// scan a line
		if !l.scanner.Scan() {
			// EOF - return nil
			return nil
		}
		// check for unexpected errors
		err := l.scanner.Err()
		if err != nil {
			log.Printf("%s\n", err)
			return nil
		}
		// get the line string
		s := l.scanner.Text()
		return &s
	} else if unsupported_term() {
		// Not a terminal we know about, so basic line reading.
		return nil
	} else {
		// A command line on stdin, our raison d'etre.
		return l.read_raw(prompt, s)
	}
}

//-----------------------------------------------------------------------------

// Call the provided function in a loop.
// Exit when the function returns true or when the exit key is pressed.
// Returns true when the loop function completes, false for early exit.
func (l *linenoise) Loop(fn func() bool, exit_key rune) bool {

	// set rawmode for stdin
	err := l.enable_rawmode(STDIN)
	if err != nil {
		log.Printf("enable rawmode error %s\n", err)
		return false
	}

	u := utf8{}
	rc := false
	looping := true

	for looping {
		// get a rune
		r := u.get_rune(STDIN, &TIMEOUT_10ms)
		if r == exit_key {
			// the loop has been cancelled
			rc = false
			looping = false
		} else {
			if fn() {
				// the loop function has completed
				rc = true
				looping = false
			}
		}
	}

	// restore the terminal mode for stdin
	l.disable_rawmode(STDIN)
	return rc
}

//-----------------------------------------------------------------------------
// Key Code Debugging

// Print scan codes on screen for debugging/development purposes
func (l *linenoise) PrintKeycodes() {

	fmt.Printf("Linenoise key codes debugging mode.\n")
	fmt.Printf("Press keys to see scan codes. Type 'quit' at any time to exit.\n")

	// set rawmode for stdin
	err := l.enable_rawmode(STDIN)
	if err != nil {
		log.Printf("enable rawmode error %s\n", err)
		return
	}

	u := utf8{}
	var cmd [4]rune
	running := true

	for running {
		// get a rune
		r := u.get_rune(STDIN, nil)
		if r == KEYCODE_NULL {
			continue
		}
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
	}

	// restore the terminal mode for stdin
	l.disable_rawmode(STDIN)
}

//-----------------------------------------------------------------------------

type Hint struct {
	Hint  string
	Color int
	Bold  bool
}

// Set the completion callback function.
func (l *linenoise) SetCompletionCallback(fn func(string) []string) {
	l.completion_callback = fn
}

// Set the hints callback function.
func (l *linenoise) SetHintsCallback(fn func(string) *Hint) {
	l.hints_callback = fn
}

// Set multiline mode
func (l *linenoise) SetMultiline(mode bool) {
	l.mlmode = mode
}

// Set the hotkey. A hotkey will cause line editing to exit.
// The hotkey will be appended to the line buffer but not displayed.
func (l *linenoise) SetHotkey(key rune) {
	l.hotkey = key
}

//-----------------------------------------------------------------------------
// Command History

// Set a history entry by index number.
func (l *linenoise) HistorySet(idx int, line string) {
	l.history[len(l.history)-1-idx] = line
}

// Get a history entry by index number.
func (l *linenoise) HistoryGet(idx int) string {
	return l.history[len(l.history)-1-idx]
}

// Return the full history list.
func (l *linenoise) HistoryList() []string {
	return l.history
}

// Return next history item.
func (l *linenoise) HistoryNext(ls *linestate) string {
	if len(l.history) == 0 {
		return ""
	}
	// update the current history entry with the line buffer
	l.HistorySet(ls.history_idx, ls.String())
	ls.history_idx -= 1
	// next history item
	if ls.history_idx < 0 {
		ls.history_idx = 0
	}
	return l.HistoryGet(ls.history_idx)
}

// Return previous history item.
func (l *linenoise) HistoryPrev(ls *linestate) string {
	if len(l.history) == 0 {
		return ""
	}
	// update the current history entry with the line buffer
	l.HistorySet(ls.history_idx, ls.String())
	ls.history_idx += 1
	// previous history item
	if ls.history_idx >= len(l.history) {
		ls.history_idx = len(l.history) - 1
	}
	return l.HistoryGet(ls.history_idx)
}

// Add a new entry to the history
func (l *linenoise) HistoryAdd(line string) {
	if l.history_maxlen == 0 {
		return
	}
	// don't add duplicate lines
	for _, s := range l.history {
		if s == line {
			return
		}
	}
	// add the line to the history
	if len(l.history) == l.history_maxlen {
		// remove the first entry
		l.history = l.history[1:]
	}
	l.history = append(l.history, line)
}

// Set the maximum length for the history.
// Truncate the current history if needed.
func (l *linenoise) HistorySetMaxlen(n int) {
	if n < 0 {
		return
	}
	l.history_maxlen = n
	current_length := len(l.history)
	if current_length > l.history_maxlen {
		// truncate and retain the latest history
		l.history = l.history[current_length-l.history_maxlen:]
	}
}

// Save the history to a file.
func (l *linenoise) HistorySave(fname string) {
	if len(l.history) == 0 {
		return
	}
	f, err := os.Create(fname)
	if err != nil {
		log.Printf("error opening %s\n", fname)
		return
	}
	_, err = f.WriteString(strings.Join(l.history, "\n"))
	if err != nil {
		log.Printf("%s error writing %s\n", fname, err)
	}
	f.Close()
}

// Load history from a file
func (l *linenoise) HistoryLoad(fname string) {
	info, err := os.Stat(fname)
	if err != nil {
		return
	}
	if !info.Mode().IsRegular() {
		log.Printf("%s is not a regular file\n", fname)
		return
	}
	f, err := os.Open(fname)
	if err != nil {
		log.Printf("%s error on open %s\n", fname, err)
		return
	}
	b := bufio.NewReader(f)
	l.history = make([]string, 0, l.history_maxlen)
	for {
		s, err := b.ReadString('\n')
		if err == nil || err == io.EOF {
			s = strings.TrimSpace(s)
			if len(s) != 0 {
				l.history = append(l.history, s)
			}
			if err == io.EOF {
				break
			}
		} else {
			log.Printf("%s error on read %s\n", fname, err)
		}
	}
	f.Close()
}

//-----------------------------------------------------------------------------
