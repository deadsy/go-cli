//-----------------------------------------------------------------------------

//-----------------------------------------------------------------------------

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/deadsy/go_linenoise/ln"
)

//-----------------------------------------------------------------------------
// cli related leaf functions

// general help
func cmd_help(ui ln.UI, args []string) {
	cli.GeneralHelp()
}

// command history
func cmd_history(ui ln.UI, args []string) {
	cli.DisplayHistory(args)
}

// exit application
func cmd_exit(ui ln.UI, args []string) {
	cli.Exit()
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

// a0 function description"""
func a0_func(ui ln.UI, args []string) {
	ui.Put(fmt.Sprintf("a0 function arguments %v\n", args))
	ui.Put("Looping... Ctrl-D to exit\n")
	loop_idx = 0
	cli.Loop(loop, ln.KEYCODE_CTRL_D)
}

// a1 function description
func a1_func(ui ln.UI, args []string) {
	ui.Put(fmt.Sprintf("a1 function arguments %v\n", args))
}

// a2 function description
func a2_func(ui ln.UI, args []string) {
	ui.Put(fmt.Sprintf("a2 function arguments %v\n", args))
}

// b0 function description
func b0_func(ui ln.UI, args []string) {
	ui.Put(fmt.Sprintf("b0 function arguments %v\n", args))
}

// b1 function description
func b1_func(ui ln.UI, args []string) {
	ui.Put(fmt.Sprintf("b1 function arguments %v\n", args))
}

// c0 function description
func c0_func(ui ln.UI, args []string) {
	ui.Put(fmt.Sprintf("c0 function arguments %v\n", args))
}

// c1 function description
func c1_func(ui ln.UI, args []string) {
	ui.Put(fmt.Sprintf("c1 function arguments %v\n", args))
}

// c2 function description
func c2_func(ui ln.UI, args []string) {
	ui.Put(fmt.Sprintf("c2 function arguments %v\n", args))
}

//-----------------------------------------------------------------------------

// example of function argument help (parm, descr)
var argument_help = []ln.Help{
	{"arg0", "arg0 description"},
	{"arg1", "arg1 description"},
	{"arg2", "arg2 description"},
}

// 'a' submenu items
var a_menu = ln.Menu{
	{"a0", a0_func, argument_help},
	{"a1", a1_func, argument_help},
	{"a2", a2_func},
}

// 'b' submenu items
var b_menu = ln.Menu{
	{"b0", b0_func, argument_help},
	{"b1", b1_func},
}

// 'c' submenu items
var c_menu = ln.Menu{
	{"c0", c0_func, argument_help},
	{"c1", c1_func, argument_help},
	{"c2", c2_func},
}

// root menu
var menu_root = ln.Menu{
	{"amenu", a_menu, "menu a functions"},
	{"bmenu", b_menu, "menu b functions"},
	{"cmenu", c_menu, "menu c functions"},
	{"exit", cmd_exit},
	{"help", cmd_help},
	{"history", cmd_history, ln.HistoryHelp},
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
var cli = ln.NewCLI(ui, "history.txt")

func main() {
	cli.SetRoot(menu_root)
	cli.SetPrompt("cli> ")
	cli.Run()
	os.Exit(0)
}

//-----------------------------------------------------------------------------
