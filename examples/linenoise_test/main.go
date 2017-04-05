//-----------------------------------------------------------------------------

//-----------------------------------------------------------------------------

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/deadsy/go_linenoise/ln"
)

//-----------------------------------------------------------------------------

const KEY_HOTKEY = '?'

//-----------------------------------------------------------------------------

// Return a list of line completions.
func completion(s string) []string {
	if len(s) >= 1 && s[0] == 'h' {
		return []string{"hello", "hello there"}
	}
	return nil
}

// Return the hints for this command.
func hints(s string) *ln.Hint {
	if s == "hello" {
		// string, color, bold
		return &ln.Hint{" World", 35, false}
	}
	return nil
}

//-----------------------------------------------------------------------------

const LOOPS = 10

var loop_idx int

// example loop function - return true on completion
func loop() bool {
	fmt.Printf("loop index %d/%d\r\n", loop_idx, LOOPS)
	time.Sleep(500 * time.Millisecond)
	loop_idx += 1
	return loop_idx > LOOPS
}

//-----------------------------------------------------------------------------

func main() {

	multiline_flag := flag.Bool("multiline", false, "enable multiline editing mode")
	keycode_flag := flag.Bool("keycodes", false, "read and display keycodes")
	loop_flag := flag.Bool("loop", false, "run a loop function with hotkey exit")
	flag.Parse()

	l := ln.NewLineNoise()

	if *multiline_flag {
		l.SetMultiline(true)
		fmt.Printf("Multi-line mode enabled.\n")
	} else if *keycode_flag {
		l.PrintKeycodes()
		os.Exit(0)
	} else if *loop_flag {
		fmt.Printf("looping: press ctrl-d to exit\n")
		rc := l.Loop(loop, ln.KEYCODE_CTRL_D)
		if rc {
			fmt.Printf("loop completed\n")
		} else {
			fmt.Printf("early exit of loop\n")
		}
		os.Exit(0)
	}

	// Set the completion callback. This will be called
	// every time the user uses the <tab> key.
	l.SetCompletionCallback(completion)
	l.SetHintsCallback(hints)

	// Load history from file. The history file is a plain text file
	// where entries are separated by newlines.
	l.HistoryLoad("history.txt")

	// Set a hotkey. A hotkey will cause the line editing to exit. The hotkey
	// will be appended to the returned line buffer but not displayed.
	l.SetHotkey(KEY_HOTKEY)

	// This is the main loop of a typical linenoise-based application.
	// The call to Read() will block until the user types something
	// and presses enter or a hotkey.
	for {
		line := l.Read("hello> ", "")
		if line == nil {
			break
		}
		s := *line
		if strings.HasPrefix(s, "/") {
			// commands
			if strings.HasPrefix(s, "/historylen") {
				/*
					        l := strings.Split(line, " ")
									if len(l) >= 2 {
										n = int(l[1], 10)
										l.HistorySetMaxlen(n)
									} else {
										fmt.Printf("no history length\n")
									}
				*/
			} else {
				fmt.Printf("unrecognized command: %s\n", s)
			}
		} else if len(s) > 0 {
			fmt.Printf("echo: '%s'\n", s)
			if strings.HasSuffix(s, string(KEY_HOTKEY)) {
				s = strings.TrimSuffix(s, string(KEY_HOTKEY))
			}
			l.HistoryAdd(s)
			l.HistorySave("history.txt")
		}
	}

	os.Exit(0)
}

//-----------------------------------------------------------------------------
