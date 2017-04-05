//-----------------------------------------------------------------------------

//-----------------------------------------------------------------------------

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/deadsy/go_linenoise/ln"
)

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

	os.Exit(0)
}

//-----------------------------------------------------------------------------
