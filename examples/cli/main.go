//-----------------------------------------------------------------------------
/*
Example code to demonstrate the Command Line Interface.
*/
//-----------------------------------------------------------------------------

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/deadsy/go-cli"
)

//-----------------------------------------------------------------------------
// cli related leaf functions

var cmdHelp = cli.Leaf{
	Descr: "general help",
	F: func(c *cli.CLI, args []string) {
		c.GeneralHelp()
	},
}

var cmdHistory = cli.Leaf{
	Descr: "command history",
	F: func(c *cli.CLI, args []string) {
		c.SetLine(c.DisplayHistory(args))
	},
}

var cmdExit = cli.Leaf{
	Descr: "exit application",
	F: func(c *cli.CLI, args []string) {
		c.Exit()
	},
}

//-----------------------------------------------------------------------------
// application leaf functions

const maxLoops = 10

var loopIndex int

// example loop function - return true on completion
func loop() bool {
	fmt.Printf("loop index %d/%d\r\n", loopIndex, maxLoops)
	time.Sleep(500 * time.Millisecond)
	loopIndex++
	return loopIndex > maxLoops
}

var a0Func = cli.Leaf{
	Descr: "a0 function description",
	F: func(c *cli.CLI, args []string) {
		c.Put(fmt.Sprintf("a0 function arguments %v\n", args))
		c.Put("Looping... Ctrl-D to exit\n")
		loopIndex = 0
		c.Loop(loop, cli.KEYCODE_CTRL_D)
	},
}

var a1Func = cli.Leaf{
	Descr: "a1 function description",
	F: func(c *cli.CLI, args []string) {
		c.Put(fmt.Sprintf("a1 function arguments %v\n", args))
	},
}

var a2Func = cli.Leaf{
	Descr: "a2 function description",
	F: func(c *cli.CLI, args []string) {
		c.Put(fmt.Sprintf("a2 function arguments %v\n", args))
	},
}

var b0Func = cli.Leaf{
	Descr: "b0 function description",
	F: func(c *cli.CLI, args []string) {
		c.Put(fmt.Sprintf("b0 function arguments %v\n", args))
	},
}

var b1Func = cli.Leaf{
	Descr: "b1 function description",
	F: func(c *cli.CLI, args []string) {
		c.Put(fmt.Sprintf("b1 function arguments %v\n", args))
	},
}

var c0Func = cli.Leaf{
	Descr: "c0 function description",
	F: func(c *cli.CLI, args []string) {
		c.Put(fmt.Sprintf("c0 function arguments %v\n", args))
	},
}

var c1Func = cli.Leaf{
	Descr: "c1 function description",
	F: func(c *cli.CLI, args []string) {
		c.Put(fmt.Sprintf("c1 function arguments %v\n", args))
	},
}

var c2Func = cli.Leaf{
	Descr: "c2 function description",
	F: func(c *cli.CLI, args []string) {
		c.Put(fmt.Sprintf("c2 function arguments %v\n", args))
	},
}

//-----------------------------------------------------------------------------

// example of function argument help (parm, descr)
var argumentHelp = []cli.Help{
	{"arg0", "arg0 description"},
	{"arg1", "arg1 description"},
	{"arg2", "arg2 description"},
}

// 'a' submenu items
var aMenu = cli.Menu{
	{"a0", a0Func, argumentHelp},
	{"a1", a1Func, argumentHelp},
	{"a2", a2Func},
}

// 'b' submenu items
var bMenu = cli.Menu{
	{"b0", b0Func, argumentHelp},
	{"b1", b1Func},
}

// 'c' submenu items
var cMenu = cli.Menu{
	{"c0", c0Func, argumentHelp},
	{"c1", c1Func, argumentHelp},
	{"c2", c2Func},
}

// root menu
var menuRoot = cli.Menu{
	{"amenu", aMenu, "menu a functions"},
	{"bmenu", bMenu, "menu b functions"},
	{"cmenu", cMenu, "menu c functions"},
	{"exit", cmdExit},
	{"help", cmdHelp},
	{"history", cmdHistory, cli.HistoryHelp},
}

//-----------------------------------------------------------------------------

// userApp is state associated with the user application.
type userApp struct {
}

// newUserApp returns a user application.
func newUserApp() *userApp {
	return &userApp{}
}

// Put outputs a string to the user application.
func (user *userApp) Put(s string) {
	fmt.Printf("%s", s)
}

//-----------------------------------------------------------------------------

func main() {
	hpath := "history.txt"
	c := cli.NewCLI(newUserApp())
	c.HistoryLoad(hpath)
	c.SetRoot(menuRoot)
	c.SetPrompt("cli> ")
	for c.Running() {
		c.Run()
	}
	c.HistorySave(hpath)
	os.Exit(0)
}

//-----------------------------------------------------------------------------
