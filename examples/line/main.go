//-----------------------------------------------------------------------------
/*
Example code to demonstrate line editing.
*/
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

	"github.com/deadsy/go-cli"
	runewidth "github.com/mattn/go-runewidth"
)

//-----------------------------------------------------------------------------

const keyHotKey = '?'
const prompt = "つのだ☆hello> "

//-----------------------------------------------------------------------------

// Return a list of line completions.
func completion(s string) []string {
	if len(s) >= 1 && s[0] == 'h' {
		return []string{"hello", "hello there"}
	}
	return nil
}

// Return the hints for this command.
func hints(s string) *cli.Hint {
	if s == "hello" {
		// string, color, bold
		return &cli.Hint{" World", 35, false}
	}
	return nil
}

//-----------------------------------------------------------------------------

const maxLoops = 10

var loopIndex int

// example loop function - return true on completion
func loop() bool {
	fmt.Printf("loop index %d/%d\r\n", loopIndex, maxLoops)
	time.Sleep(500 * time.Millisecond)
	loopIndex++
	return loopIndex > maxLoops
}

//-----------------------------------------------------------------------------

func main() {

	multilineFlag := flag.Bool("multiline", false, "enable multiline editing mode")
	keycodeFlag := flag.Bool("keycodes", false, "read and display keycodes")
	loopFlag := flag.Bool("loop", false, "run a loop function with hotkey exit")
	flag.Parse()

	l := cli.NewLineNoise()

	if *multilineFlag {
		l.SetMultiline(true)
		fmt.Printf("Multi-line mode enabled.\n")
	} else if *keycodeFlag {
		l.PrintKeycodes()
		os.Exit(0)
	} else if *loopFlag {
		fmt.Printf("looping: press ctrl-d to exit\n")
		rc := l.Loop(loop, cli.KEYCODE_CTRL_D)
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
	l.SetHotkey(keyHotKey)

	// This is the main loop of a typical linenoise-based application.
	// The call to Read() will block until the user types something
	// and presses enter or a hotkey.
	for {
		s, err := l.Read(prompt, "")
		if err != nil {
			if err == cli.QUIT {
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
			fmt.Printf("echo: '%s' %d cols\n", s, runewidth.StringWidth(s))
			if strings.HasSuffix(s, string(keyHotKey)) {
				s = strings.TrimSuffix(s, string(keyHotKey))
			}
			l.HistoryAdd(s)
			l.HistorySave("history.txt")
		}
	}
	os.Exit(0)
}

//-----------------------------------------------------------------------------
