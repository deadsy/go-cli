//-----------------------------------------------------------------------------
/*

Line Editing

See: https://github.com/deadsy/go-cli

Based on: http://github.com/antirez/linenoise

Notes on Unicode: This codes operates on UTF8 codepoints. It assumes each glyph
occupies k columns, where k is an integer >= 0. It assumes the runewidth
call will tell us the number of columns taken by a UTF8 string. These assumptions
won't be true for all character sets. If you don't have a monospaced version of the
character being rendered then these assumptions will fail and odd things will
be seen.

*/
//-----------------------------------------------------------------------------

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unsafe"

	"github.com/creack/termios/raw"
	"github.com/deadsy/go-fdset"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
)

//-----------------------------------------------------------------------------

// Keycodes
const (
	KeycodeNull  = 0
	KeycodeCtrlA = 1
	KeycodeCtrlB = 2
	KeycodeCtrlC = 3
	KeycodeCtrlD = 4
	KeycodeCtrlE = 5
	KeycodeCtrlF = 6
	KeycodeCtrlH = 8
	KeycodeTAB   = 9
	KeycodeLF    = 10
	KeycodeCtrlK = 11
	KeycodeCtrlL = 12
	KeycodeCR    = 13
	KeycodeCtrlN = 14
	KeycodeCtrlP = 16
	KeycodeCtrlT = 20
	KeycodeCtrlU = 21
	KeycodeCtrlW = 23
	KeycodeESC   = 27
	KeycodeBS    = 127
)

var timeout20ms = syscall.Timeval{0, 20 * 1000}
var timeout10ms = syscall.Timeval{0, 10 * 1000}

// ErrQuit is returned when the user has quit line editing.
var ErrQuit = errors.New("quit")

//-----------------------------------------------------------------------------

// boolean to integer
func btoi(x bool) int {
	if x {
		return 1
	}
	return 0
}

//-----------------------------------------------------------------------------
// control the terminal mode

// Set a tty terminal to raw mode.
func setRawMode(fd int) (*raw.Termios, error) {
	// make sure this is a tty
	if !isatty.IsTerminal(uintptr(fd)) {
		return nil, fmt.Errorf("fd %d is not a tty", fd)
	}
	// get the terminal IO mode
	originalMode, err := raw.TcGetAttr(uintptr(fd))
	if err != nil {
		return nil, err
	}
	// modify the original mode
	newMode := *originalMode
	newMode.Iflag &^= (syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON)
	newMode.Oflag &^= syscall.OPOST
	newMode.Lflag &^= (syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN)
	newMode.Cflag &^= (syscall.CSIZE | syscall.PARENB)
	newMode.Cflag |= syscall.CS8
	newMode.Cc[syscall.VMIN] = 1
	newMode.Cc[syscall.VTIME] = 0
	err = raw.TcSetAttr(uintptr(fd), &newMode)
	if err != nil {
		return nil, err
	}
	return originalMode, nil
}

// Restore the terminal mode.
func restoreMode(fd int, mode *raw.Termios) error {
	return raw.TcSetAttr(uintptr(fd), mode)
}

//-----------------------------------------------------------------------------
// UTF8 Decoding

const (
	getByte0 = iota
	get3More
	get2More
	get1More
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
	case getByte0:
		if c&0x80 == 0 {
			// 1 byte
			return rune(c), 1
		} else if c&0xe0 == 0xc0 {
			// 2 byte
			u.val = int32(c&0x1f) << 6
			u.count = 2
			u.state = get1More
			return KeycodeNull, 0
		} else if c&0xf0 == 0xe0 {
			// 3 bytes
			u.val = int32(c&0x0f) << 6
			u.count = 3
			u.state = get2More
			return KeycodeNull, 0
		} else if c&0xf8 == 0xf0 {
			// 4 bytes
			u.val = int32(c&0x07) << 6
			u.count = 4
			u.state = get3More
			return KeycodeNull, 0
		}
	case get3More:
		if c&0xc0 == 0x80 {
			u.state = get2More
			u.val |= int32(c & 0x3f)
			u.val <<= 6
			return KeycodeNull, 0
		}
	case get2More:
		if c&0xc0 == 0x80 {
			u.state = get1More
			u.val |= int32(c & 0x3f)
			u.val <<= 6
			return KeycodeNull, 0
		}
	case get1More:
		if c&0xc0 == 0x80 {
			u.state = getByte0
			u.val |= int32(c & 0x3f)
			return rune(u.val), u.count
		}
	}
	// Error
	u.state = getByte0
	return unicode.ReplacementChar, 1
}

// read a single rune from a file descriptor (with timeout)
// timeout >= 0 : wait for timeout seconds
// timeout = nil : return immediately
func (u *utf8) getRune(fd int, timeout *syscall.Timeval) rune {
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
			return KeycodeNull
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
		return KeycodeNull
	}
	if size == 1 && r == unicode.ReplacementChar {
		// utf8 decode error
		return KeycodeNull
	}
	return r
}

//-----------------------------------------------------------------------------

// If fd is not readable within the timeout period return true.
func would_block(fd int, timeout *syscall.Timeval) bool {
	rd := syscall.FdSet{}
	fdset.Set(fd, &rd)
	n, err := syscall.Select(fd+1, &rd, nil, nil, timeout)
	if err != nil {
		log.Printf("select error %s\n", err)
		return false
	}
	return n == 0
}

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
const defaultCols = 80

// Get the horizontal cursor position
func get_cursor_position(ifd, ofd int) int {
	// query the cursor location
	if puts(ofd, "\x1b[6n") != 4 {
		return -1
	}
	// read the response: ESC [ rows ; cols R
	// rows/cols are decimal number strings
	buf := make([]rune, 0, 32)
	u := utf8{}

	for len(buf) < 32 {
		r := u.getRune(ifd, &timeout20ms)
		if r == KeycodeNull {
			break
		}
		buf = append(buf, r)
		if r == 'R' {
			break
		}
	}
	// parse it: esc [ number ; number R (at least 6 characters)
	if len(buf) < 6 || buf[0] != KeycodeESC || buf[1] != '[' || buf[len(buf)-1] != 'R' {
		return -1
	}
	// should have 2 number fields
	x := strings.Split(string(buf[2:len(buf)-1]), ";")
	if len(x) != 2 {
		return -1
	}
	// return the cols
	cols, err := strconv.Atoi(x[1])
	if err != nil {
		return -1
	}
	return cols
}

// Get the number of columns for the terminal. Assume defaultCols if it fails.
func getColumns(ifd, ofd int) int {
	// try using the ioctl to get the number of cols
	var winsize [4]uint16
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdout), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&winsize)))
	if err == 0 {
		return int(winsize[1])
	}
	// the ioctl failed - try using the terminal itself
	start := get_cursor_position(ifd, ofd)
	if start < 0 {
		return defaultCols
	}
	// Go to right margin and get position
	if puts(ofd, "\x1b[999C") != 6 {
		return defaultCols
	}
	cols := get_cursor_position(ifd, ofd)
	if cols < 0 {
		return defaultCols
	}
	// restore the position
	if cols > start {
		puts(ofd, fmt.Sprintf("\x1b[%dD", cols-start))
	}
	return cols
}

//-----------------------------------------------------------------------------

// Clear the screen.
func clear_screen() {
	puts(syscall.Stdout, "\x1b[H\x1b[2J")
}

// Beep.
func beep() {
	puts(syscall.Stderr, "\x07")
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
	ifd, ofd     int        // stdin/stdout file descriptors
	prompt       string     // prompt string
	prompt_width int        // prompt width in terminal columns
	ts           *Linenoise // terminal state
	history_idx  int        // history index we are currently editing, 0 is the LAST entry
	buf          []rune     // line buffer
	cols         int        // number of columns in terminal
	pos          int        // current cursor position within line buffer
	oldpos       int        // previous refresh cursor position (multiline)
	maxrows      int        // maximum num of rows used so far (multiline)
}

func newLineState(ifd, ofd int, prompt string, ts *Linenoise) *linestate {
	ls := linestate{}
	ls.ifd = ifd
	ls.ofd = ofd
	ls.prompt = prompt
	ls.prompt_width = runewidth.StringWidth(prompt)
	ls.ts = ts
	ls.cols = getColumns(ifd, ofd)
	return &ls
}

// show hints to the right of the cursor
func (ls *linestate) refresh_show_hints() []string {
	// do we have a hints callback?
	if ls.ts.hints_callback == nil {
		// no hints
		return nil
	}
	// How many columns do we have for the hint?
	hint_cols := ls.cols - ls.prompt_width - runewidth.StringWidth(string(ls.buf))
	if hint_cols <= 0 {
		// no space to display hints
		return nil
	}
	// get the hint
	h := ls.ts.hints_callback(string(ls.buf))
	if h == nil || len(h.Hint) == 0 {
		// no hints
		return nil
	}
	// trim the hint until it fits
	hEnd := len(h.Hint)
	for runewidth.StringWidth(h.Hint[:hEnd]) > hint_cols {
		hEnd--
	}
	// color fixup
	if h.Bold && h.Color < 0 {
		h.Color = 37
	}
	// build the output string
	seq := make([]string, 0, 3)
	if h.Color >= 0 || h.Bold {
		seq = append(seq, fmt.Sprintf("\033[%d;%d;49m", btoi(h.Bold), h.Color))
	}
	seq = append(seq, h.Hint[:hEnd])
	if h.Color >= 0 || h.Bold {
		seq = append(seq, "\033[0m")
	}
	return seq
}

// single line refresh
func (ls *linestate) refresh_singleline() {
	// indices within buffer to be rendered
	b_start := 0
	b_end := len(ls.buf)
	// trim the left hand side to keep the cursor position on the screen
	pos_width := runewidth.StringWidth(string(ls.buf[:ls.pos]))
	for ls.prompt_width+pos_width >= ls.cols {
		b_start++
		pos_width = runewidth.StringWidth(string(ls.buf[b_start:ls.pos]))
	}
	// trim the right hand side - don't print beyond max columns
	buf_width := runewidth.StringWidth(string(ls.buf[b_start:b_end]))
	for ls.prompt_width+buf_width >= ls.cols {
		b_end--
		buf_width = runewidth.StringWidth(string(ls.buf[b_start:b_end]))
	}
	// build the output string
	seq := make([]string, 0, 6)
	// cursor to the left edge
	seq = append(seq, "\r")
	// write the prompt
	seq = append(seq, ls.prompt)
	// write the current buffer content
	seq = append(seq, string(ls.buf[b_start:b_end]))
	// Show hints (if any)
	seq = append(seq, ls.refresh_show_hints()...)
	// Erase to right
	seq = append(seq, "\x1b[0K")
	// Move cursor to original position
	seq = append(seq, fmt.Sprintf("\r\x1b[%dC", ls.prompt_width+pos_width))
	// write it out
	puts(ls.ofd, strings.Join(seq, ""))
}

// multiline refresh
func (ls *linestate) refresh_multiline() {
	buf_width := runewidth.StringWidth(string(ls.buf))
	old_rows := ls.maxrows
	// cursor position relative to row
	rpos := (ls.prompt_width + ls.oldpos + ls.cols) / ls.cols
	// rows used by current buf
	rows := (ls.prompt_width + buf_width + ls.cols - 1) / ls.cols
	// Update maxrows if needed
	if rows > ls.maxrows {
		ls.maxrows = rows
	}
	// build the output string
	seq := make([]string, 0, 15)
	// First step: clear all the lines used before. To do so start by going to the last row.
	if old_rows-rpos > 0 {
		seq = append(seq, fmt.Sprintf("\x1b[%dB", old_rows-rpos))
	}
	// Now for every row clear it, go up.
	for j := 0; j < old_rows-1; j++ {
		seq = append(seq, "\r\x1b[0K\x1b[1A")
	}
	// Clear the top line.
	seq = append(seq, "\r\x1b[0K")
	// Write the prompt and the current buffer content
	seq = append(seq, ls.prompt)
	seq = append(seq, string(ls.buf))
	// Show hints (if any)
	seq = append(seq, ls.refresh_show_hints()...)
	// If we are at the very end of the screen with our prompt, we need to
	// emit a newline and move the prompt to the first column.
	if ls.pos != 0 && ls.pos == buf_width && (ls.pos+ls.prompt_width)%ls.cols == 0 {
		seq = append(seq, "\n\r")
		rows++
		if rows > ls.maxrows {
			ls.maxrows = rows
		}
	}
	// Move cursor to right position.
	rpos2 := (ls.prompt_width + ls.pos + ls.cols) / ls.cols // current cursor relative row.
	// Go up till we reach the expected positon.
	if rows-rpos2 > 0 {
		seq = append(seq, fmt.Sprintf("\x1b[%dA", rows-rpos2))
	}
	// Set column
	col := (ls.prompt_width + ls.pos) % ls.cols
	if col != 0 {
		seq = append(seq, fmt.Sprintf("\r\x1b[%dC", col))
	} else {
		seq = append(seq, "\r")
	}
	// save the cursor position
	ls.oldpos = ls.pos
	// write it out
	puts(ls.ofd, strings.Join(seq, ""))
}

// refresh the edit line
func (ls *linestate) refresh_line() {
	if ls.ts.mlmode {
		ls.refresh_multiline()
	} else {
		ls.refresh_singleline()
	}
}

// delete the character at the current cursor position
func (ls *linestate) editDelete() {
	if len(ls.buf) > 0 && ls.pos < len(ls.buf) {
		ls.buf = append(ls.buf[:ls.pos], ls.buf[ls.pos+1:]...)
		ls.refresh_line()
	}
}

// delete the character to the left of the current cursor position
func (ls *linestate) editBackspace() {
	if ls.pos > 0 && len(ls.buf) > 0 {
		ls.buf = append(ls.buf[:ls.pos-1], ls.buf[ls.pos:]...)
		ls.pos--
		ls.refresh_line()
	}
}

// insert a character at the current cursor position
func (ls *linestate) editInsert(r rune) {
	ls.buf = append(ls.buf[:ls.pos], append([]rune{r}, ls.buf[ls.pos:]...)...)
	ls.pos++
	ls.refresh_line()
}

// Swap current character with the previous character.
func (ls *linestate) editSwap() {
	if ls.pos > 0 && ls.pos < len(ls.buf) {
		tmp := ls.buf[ls.pos-1]
		ls.buf[ls.pos-1] = ls.buf[ls.pos]
		ls.buf[ls.pos] = tmp
		if ls.pos != len(ls.buf)-1 {
			ls.pos++
		}
		ls.refresh_line()
	}
}

// Set the line buffer to a string.
func (ls *linestate) editSet(s string) {
	ls.buf = []rune(s)
	ls.pos = len(ls.buf)
	ls.refresh_line()
}

// Move cursor on the left.
func (ls *linestate) editMoveLeft() {
	if ls.pos > 0 {
		ls.pos--
		ls.refresh_line()
	}
}

// Move cursor to the right.
func (ls *linestate) editMoveRight() {
	if ls.pos != len(ls.buf) {
		ls.pos++
		ls.refresh_line()
	}
}

// Move to the start of the line buffer.
func (ls *linestate) editMoveHome() {
	if ls.pos > 0 {
		ls.pos = 0
		ls.refresh_line()
	}
}

// Move to the end of the line buffer.
func (ls *linestate) editMoveEnd() {
	if ls.pos != len(ls.buf) {
		ls.pos = len(ls.buf)
		ls.refresh_line()
	}
}

// Delete the line.
func (ls *linestate) delete_line() {
	ls.buf = nil // []rune{}
	ls.pos = 0
	ls.refresh_line()
}

// Delete from the current cursor postion to the end of the line.
func (ls *linestate) delete_to_end() {
	ls.buf = ls.buf[:ls.pos]
	ls.refresh_line()
}

// Delete the previous space delimited word.
func (ls *linestate) delete_prev_word() {
	old_pos := ls.pos
	// remove spaces
	for ls.pos > 0 && ls.buf[ls.pos-1] == ' ' {
		ls.pos--
	}
	// remove word
	for ls.pos > 0 && ls.buf[ls.pos-1] != ' ' {
		ls.pos--
	}
	ls.buf = append(ls.buf[:ls.pos], ls.buf[old_pos:]...)
	ls.refresh_line()
}

// Show completions for the current line.
func (ls *linestate) complete_line() rune {
	// get a list of line completions
	lc := ls.ts.completion_callback(ls.String())
	if len(lc) == 0 {
		// no line completions
		beep()
		return KeycodeNull
	}
	// navigate and display the line completions
	stop := false
	idx := 0
	u := utf8{}
	var r rune
	for !stop {
		if idx < len(lc) {
			// save the line buffer
			saved_buf := ls.buf
			saved_pos := ls.pos
			// show the completion
			ls.buf = []rune(lc[idx])
			ls.pos = len(ls.buf)
			ls.refresh_line()
			// restore the line buffer
			ls.buf = saved_buf
			ls.pos = saved_pos
		} else {
			// show the original buffer
			ls.refresh_line()
		}
		// navigate through the completions
		r = u.getRune(ls.ifd, nil)
		if r == KeycodeNull {
			// error on read
			stop = true
		} else if r == KeycodeTAB {
			// loop through the completions
			idx = (idx + 1) % (len(lc) + 1)
			if idx == len(lc) {
				beep()
			}
		} else if r == KeycodeESC {
			// could be an escape, could be an escape sequence
			if would_block(ls.ifd, &timeout20ms) {
				// nothing more to read, looks like a single escape
				// re-show the original buffer
				if idx < len(lc) {
					ls.refresh_line()
				}
				// don't pass the escape key back
				r = KeycodeNull
			} else {
				// probably an escape sequence
				// update the buffer and return
				if idx < len(lc) {
					ls.buf = []rune(lc[idx])
					ls.pos = len(ls.buf)
				}
			}
			stop = true
		} else {
			// update the buffer and return
			if idx < len(lc) {
				ls.buf = []rune(lc[idx])
				ls.pos = len(ls.buf)
			}
			stop = true
		}
	}
	// return the last rune read
	return r
}

// Return a string for the current line buffer.
func (ls *linestate) String() string {
	return string(ls.buf)
}

//-----------------------------------------------------------------------------

// Linenoise stores line editor state.
type Linenoise struct {
	history             []string              // list of history strings
	history_maxlen      int                   // maximum number of history entries
	rawmode             bool                  // are we in raw mode?
	mlmode              bool                  // are we in multiline mode?
	savedmode           *raw.Termios          // saved terminal mode
	completion_callback func(string) []string // callback function for tab completion
	hints_callback      func(string) *Hint    // callback function for hints
	hotkey              rune                  // character for hotkey
	scanner             *bufio.Scanner        // buffered IO scanner for file reading
}

// NewLineNoise returns a new line editor.
func NewLineNoise() *Linenoise {
	l := Linenoise{}
	l.history_maxlen = 32
	return &l
}

// Enable raw mode
func (l *Linenoise) enableRawMode(fd int) error {
	mode, err := setRawMode(fd)
	if err != nil {
		return err
	}
	l.rawmode = true
	l.savedmode = mode
	return nil
}

// Disable raw mode
func (l *Linenoise) disableRawMode(fd int) error {
	if l.rawmode {
		err := restoreMode(fd, l.savedmode)
		if err != nil {
			return err
		}
	}
	l.rawmode = false
	return nil
}

//-----------------------------------------------------------------------------

// edit a line in raw mode
func (l *Linenoise) edit(ifd, ofd int, prompt, init string) (string, error) {
	// create the line state
	ls := newLineState(ifd, ofd, prompt, l)
	// set and output the initial line
	ls.editSet(init)
	// The latest history entry is always our current buffer
	l.HistoryAdd(ls.String())

	u := utf8{}

	for {
		r := u.getRune(syscall.Stdin, nil)
		if r == KeycodeNull {
			continue
		}
		// Autocomplete when the callback is set.
		// It returns the character to be handled next.
		if r == KeycodeTAB && l.completion_callback != nil {
			r = ls.complete_line()
			if r == KeycodeNull {
				continue
			}
		}
		if r == KeycodeCR || r == l.hotkey {
			l.historyPop(-1)
			if l.hints_callback != nil {
				// Refresh the line without hints to leave the
				// line as the user typed it after the newline.
				hcb := l.hints_callback
				l.hints_callback = nil
				ls.refresh_line()
				l.hints_callback = hcb
			}
			s := ls.String()
			if r == l.hotkey {
				return s + string(l.hotkey), nil
			}
			return s, nil
		} else if r == KeycodeBS {
			// backspace: remove the character to the left of the cursor
			ls.editBackspace()

		} else if r == KeycodeESC {
			if would_block(ifd, &timeout20ms) {
				// looks like a single escape- abandon the line
				l.historyPop(-1)
				return "", nil
			}
			// escape sequence
			s0 := u.getRune(ifd, &timeout20ms)
			s1 := u.getRune(ifd, &timeout20ms)
			if s0 == '[' {
				// ESC [ sequence
				if s1 >= '0' && s1 <= '9' {
					// Extended escape, read additional byte.
					s2 := u.getRune(ifd, &timeout20ms)
					if s2 == '~' {
						if s1 == '3' {
							// delete
							ls.editDelete()
						}
					}
				} else {
					if s1 == 'A' {
						// cursor up
						ls.editSet(l.historyPrev(ls))
					} else if s1 == 'B' {
						// cursor down
						ls.editSet(l.historyNext(ls))
					} else if s1 == 'C' {
						// cursor right
						ls.editMoveRight()
					} else if s1 == 'D' {
						// cursor left
						ls.editMoveLeft()
					} else if s1 == 'H' {
						// cursor home
						ls.editMoveHome()
					} else if s1 == 'F' {
						// cursor end
						ls.editMoveEnd()
					}
				}
			} else if s0 == '0' {
				// ESC 0 sequence
				if s1 == 'H' {
					// cursor home
					ls.editMoveHome()
				} else if s1 == 'F' {
					// cursor end
					ls.editMoveEnd()
				}
			}
		} else if r == KeycodeCtrlA {
			// go to the start of the line
			ls.editMoveHome()
		} else if r == KeycodeCtrlB {
			// cursor left
			ls.editMoveLeft()
		} else if r == KeycodeCtrlC {
			// return QUIT
			return "", ErrQuit
		} else if r == KeycodeCtrlD {
			if len(ls.buf) > 0 {
				// delete: remove the character to the right of the cursor.
				ls.editDelete()
			} else {
				// nothing to delete - QUIT
				l.historyPop(-1)
				return "", ErrQuit
			}
		} else if r == KeycodeCtrlE {
			// go to the end of the line
			ls.editMoveEnd()
		} else if r == KeycodeCtrlF {
			// cursor right
			ls.editMoveRight()
		} else if r == KeycodeCtrlH {
			// backspace: remove the character to the left of the cursor
			ls.editBackspace()
		} else if r == KeycodeCtrlK {
			// delete to the end of the line
			ls.delete_to_end()
		} else if r == KeycodeCtrlL {
			// clear screen
			clear_screen()
			ls.refresh_line()
		} else if r == KeycodeCtrlN {
			// next history item
			ls.editSet(l.historyNext(ls))
		} else if r == KeycodeCtrlP {
			// previous history item
			ls.editSet(l.historyPrev(ls))
		} else if r == KeycodeCtrlT {
			// swap current character with the previous
			ls.editSwap()
		} else if r == KeycodeCtrlU {
			// delete the whole line
			ls.delete_line()
		} else if r == KeycodeCtrlW {
			// delete previous word
			ls.delete_prev_word()
		} else {
			// insert the character into the line buffer
			ls.editInsert(r)
		}
	}
}

//-----------------------------------------------------------------------------

// Read a line from stdin in raw mode.
func (l *Linenoise) read_raw(prompt, init string) (string, error) {
	// set rawmode for stdin
	l.enableRawMode(syscall.Stdin)
	defer l.disableRawMode(syscall.Stdin)
	// edit the line
	s, err := l.edit(syscall.Stdin, syscall.Stdout, prompt, init)
	fmt.Printf("\r\n")
	return s, err
}

// Read a line using basic buffered IO.
func (l *Linenoise) read_basic() (string, error) {
	if l.scanner == nil {
		l.scanner = bufio.NewScanner(os.Stdin)
	}
	// scan a line
	if !l.scanner.Scan() {
		// EOF - return quit
		return "", ErrQuit
	}
	// check for unexpected errors
	err := l.scanner.Err()
	if err != nil {
		return "", err
	}
	// get the line string
	return l.scanner.Text(), nil
}

// Read a line. Return nil on EOF/quit.
func (l *Linenoise) Read(prompt, init string) (string, error) {
	if !isatty.IsTerminal(uintptr(syscall.Stdin)) {
		// Not a tty, read from a file or pipe.
		return l.read_basic()
	} else if unsupported_term() {
		// Not a terminal we know about, so basic line reading.
		fmt.Printf(prompt)
		s, err := l.read_basic()
		if err == ErrQuit {
			fmt.Printf("\n")
		}
		return s, err
	} else {
		// A command line on stdin, our raison d'etre.
		return l.read_raw(prompt, init)
	}
}

//-----------------------------------------------------------------------------

// Loop calls the provided function in a loop.
// Exit when the function returns true or when the exit key is pressed.
// Returns true when the loop function completes, false for early exit.
func (l *Linenoise) Loop(fn func() bool, exit_key rune) bool {

	// set rawmode for stdin
	err := l.enableRawMode(syscall.Stdin)
	if err != nil {
		log.Printf("enable rawmode error %s\n", err)
		return false
	}

	u := utf8{}
	rc := false
	looping := true

	for looping {
		// get a rune
		r := u.getRune(syscall.Stdin, &timeout10ms)
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
	l.disableRawMode(syscall.Stdin)
	return rc
}

//-----------------------------------------------------------------------------
// Key Code Debugging

// PrintKeycodes prints scan codes on the screen for debugging/development purposes.
func (l *Linenoise) PrintKeycodes() {

	fmt.Printf("Linenoise key codes debugging mode.\n")
	fmt.Printf("Press keys to see scan codes. Type 'quit' at any time to exit.\n")

	// set rawmode for stdin
	err := l.enableRawMode(syscall.Stdin)
	if err != nil {
		log.Printf("enable rawmode error %s\n", err)
		return
	}

	u := utf8{}
	var cmd [4]rune
	running := true

	for running {
		// get a rune
		r := u.getRune(syscall.Stdin, nil)
		if r == KeycodeNull {
			continue
		}
		// display the character
		var s string
		if unicode.IsPrint(r) {
			s = string(r)
		} else {
			switch r {
			case KeycodeCR:
				s = "\\r"
			case KeycodeTAB:
				s = "\\t"
			case KeycodeESC:
				s = "ESC"
			case KeycodeLF:
				s = "\\n"
			case KeycodeBS:
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
	l.disableRawMode(syscall.Stdin)
}

//-----------------------------------------------------------------------------

// Hint is used to provide hint information to the line editor.
type Hint struct {
	Hint  string
	Color int
	Bold  bool
}

// SetCompletionCallback sets the completion callback function.
func (l *Linenoise) SetCompletionCallback(fn func(string) []string) {
	l.completion_callback = fn
}

// SetHintsCallback sets the hints callback function.
func (l *Linenoise) SetHintsCallback(fn func(string) *Hint) {
	l.hints_callback = fn
}

// SetMultiline sets multiline editing mode.
func (l *Linenoise) SetMultiline(mode bool) {
	l.mlmode = mode
}

// SetHotkey sets the hotkey that causes line editing to exit.
// The hotkey will be appended to the line buffer but not displayed.
func (l *Linenoise) SetHotkey(key rune) {
	l.hotkey = key
}

//-----------------------------------------------------------------------------
// Command History

// pop an entry from the history list
func (l *Linenoise) historyPop(idx int) string {
	if idx < 0 {
		// pop the last entry
		idx = len(l.history) - 1
	}
	if idx >= 0 && idx < len(l.history) {
		s := l.history[idx]
		l.history = append(l.history[:idx], l.history[idx+1:]...)
		return s
	}
	// nothing to pop
	return ""
}

// Set a history entry by index number.
func (l *Linenoise) historySet(idx int, line string) {
	l.history[len(l.history)-1-idx] = line
}

// Get a history entry by index number.
func (l *Linenoise) historyGet(idx int) string {
	return l.history[len(l.history)-1-idx]
}

// Return the full history list.
func (l *Linenoise) history_list() []string {
	return l.history
}

// Return next history item.
func (l *Linenoise) historyNext(ls *linestate) string {
	if len(l.history) == 0 {
		return ""
	}
	// update the current history entry with the line buffer
	l.historySet(ls.history_idx, ls.String())
	ls.history_idx--
	// next history item
	if ls.history_idx < 0 {
		ls.history_idx = 0
	}
	return l.historyGet(ls.history_idx)
}

// Return previous history item.
func (l *Linenoise) historyPrev(ls *linestate) string {
	if len(l.history) == 0 {
		return ""
	}
	// update the current history entry with the line buffer
	l.historySet(ls.history_idx, ls.String())
	ls.history_idx++
	// previous history item
	if ls.history_idx >= len(l.history) {
		ls.history_idx = len(l.history) - 1
	}
	return l.historyGet(ls.history_idx)
}

// HistoryAdd adds a new entry to the history.
func (l *Linenoise) HistoryAdd(line string) {
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
		l.historyPop(0)
	}
	l.history = append(l.history, line)
}

// HistorySetMaxlen sets the maximum length for the history.
// Truncate the current history if needed.
func (l *Linenoise) HistorySetMaxlen(n int) {
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

// HistorySave saves the history to a file.
func (l *Linenoise) HistorySave(fname string) {
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

// HistoryLoad loads history from a file.
func (l *Linenoise) HistoryLoad(fname string) {
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
