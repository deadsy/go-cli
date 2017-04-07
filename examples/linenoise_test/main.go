//-----------------------------------------------------------------------------

//-----------------------------------------------------------------------------

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deadsy/go_linenoise/ln"
	runewidth "github.com/mattn/go-runewidth"
)

//-----------------------------------------------------------------------------

const KEY_HOTKEY = '?'

//const PROMPT = "Կրնմमैंकाँखा Hello> "
const PROMPT = "Hello> "

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

	s0 := "खखखखखB"
	s1 := "AAAAAB"
	s2 := "한한한한한B"
	s3 := "01234567890"
	s4 := "⌨⌨⌨⌨⌨B"

	fmt.Printf("%s %d\n", s0, runewidth.StringWidth(s0))
	fmt.Printf("%s %d\n", s1, runewidth.StringWidth(s1))
	fmt.Printf("%s %d\n", s2, runewidth.StringWidth(s2))
	fmt.Printf("%s %d\n", s3, runewidth.StringWidth(s3))
	fmt.Printf("%s %d\n", s4, runewidth.StringWidth(s4))

	os.Exit(0)

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
		s, err := l.Read(PROMPT, "")
		if err != nil {
			if err == ln.QUIT {
				break
			}
			log.Printf("%s\n", err)
			break
		}
		if strings.HasPrefix(s, "/") {
			// commands
			if strings.HasPrefix(s, "/historylen") {
				args := strings.Fields(s)
				if len(args) >= 2 {
					n, err := strconv.Atoi(args[1])
					if err == nil {
						l.HistorySetMaxlen(n)
					} else {
						fmt.Printf("invalid history length\n")
					}
				} else {
					fmt.Printf("no history length\n")
				}
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
