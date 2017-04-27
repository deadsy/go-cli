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

var cmd_help = cli.Leaf{
	Descr: "general help",
	F: func(ui cli.UI, args []string) {
		app.GeneralHelp()
	},
}

var cmd_history = cli.Leaf{
	Descr: "command history",
	F: func(ui cli.UI, args []string) {
		s := app.DisplayHistory(args)
		app.SetLine(s)
	},
}

var cmd_exit = cli.Leaf{
	Descr: "exit application",
	F: func(ui cli.UI, args []string) {
		app.Exit()
	},
}

//-----------------------------------------------------------------------------
// application leaf functions

const LOOPS = 10

var loop_idx int

// example loop function - return true on completion
func loop() bool {
	fmt.Printf("loop index %d/%d\r\n", loop_idx, LOOPS)
	time.Sleep(500 * time.Millisecond)
	loop_idx += 1
	return loop_idx > LOOPS
}

var a0_func = cli.Leaf{
	Descr: "a0 function description",
	F: func(ui cli.UI, args []string) {
		ui.Put(fmt.Sprintf("a0 function arguments %v\n", args))
		ui.Put("Looping... Ctrl-D to exit\n")
		loop_idx = 0
		app.Loop(loop, cli.KEYCODE_CTRL_D)
	},
}

var a1_func = cli.Leaf{
	Descr: "a1 function description",
	F: func(ui cli.UI, args []string) {
		ui.Put(fmt.Sprintf("a1 function arguments %v\n", args))
	},
}

var a2_func = cli.Leaf{
	Descr: "a2 function description",
	F: func(ui cli.UI, args []string) {
		ui.Put(fmt.Sprintf("a2 function arguments %v\n", args))
	},
}

var b0_func = cli.Leaf{
	Descr: "b0 function description",
	F: func(ui cli.UI, args []string) {
		ui.Put(fmt.Sprintf("b0 function arguments %v\n", args))
	},
}

var b1_func = cli.Leaf{
	Descr: "b1 function description",
	F: func(ui cli.UI, args []string) {
		ui.Put(fmt.Sprintf("b1 function arguments %v\n", args))
	},
}

var c0_func = cli.Leaf{
	Descr: "c0 function description",
	F: func(ui cli.UI, args []string) {
		ui.Put(fmt.Sprintf("c0 function arguments %v\n", args))
	},
}

var c1_func = cli.Leaf{
	Descr: "c1 function description",
	F: func(ui cli.UI, args []string) {
		ui.Put(fmt.Sprintf("c1 function arguments %v\n", args))
	},
}

var c2_func = cli.Leaf{
	Descr: "c2 function description",
	F: func(ui cli.UI, args []string) {
		ui.Put(fmt.Sprintf("c2 function arguments %v\n", args))
	},
}

//-----------------------------------------------------------------------------

// example of function argument help (parm, descr)
var argument_help = []cli.Help{
	{"arg0", "arg0 description"},
	{"arg1", "arg1 description"},
	{"arg2", "arg2 description"},
}

// 'a' submenu items
var a_menu = cli.Menu{
	{"a0", a0_func, argument_help},
	{"a1", a1_func, argument_help},
	{"a2", a2_func},
}

// 'b' submenu items
var b_menu = cli.Menu{
	{"b0", b0_func, argument_help},
	{"b1", b1_func},
}

// 'c' submenu items
var c_menu = cli.Menu{
	{"c0", c0_func, argument_help},
	{"c1", c1_func, argument_help},
	{"c2", c2_func},
}

// root menu
var menu_root = cli.Menu{
	{"amenu", a_menu, "menu a functions"},
	{"bmenu", b_menu, "menu b functions"},
	{"cmenu", c_menu, "menu c functions"},
	{"exit", cmd_exit},
	{"help", cmd_help},
	{"history", cmd_history, cli.HistoryHelp},
}

//-----------------------------------------------------------------------------

type user_interface struct {
}

func NewUI() *user_interface {
	ui := user_interface{}
	return &ui
}

func (ui *user_interface) Put(s string) {
	fmt.Printf("%s", s)
}

//-----------------------------------------------------------------------------
// setup the cli object

var ui = NewUI()
var app = cli.NewCLI(ui, "history.txt")

func main() {
	app.SetRoot(menu_root)
	app.SetPrompt("cli> ")
	app.Run()
	os.Exit(0)
}

//-----------------------------------------------------------------------------
