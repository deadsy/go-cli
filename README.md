# go-cli

A Go based line editor and command line interface.

The line editing code is a port of the C based linenoise library.

See: http://github.com/antirez/linenoise

The line editor can be used standalone. The CLI makes use of the line editor.

## Line Editing Features
 * single line editing
 * multiline editing
 * input from files/pipes
 * input from unsupported terminals
 * history
 * completions
 * hints
 * line buffer initialization: Set an initial buffer string for editing.
 * hot keys: Set a special hot key for exiting line editing.
 * loop functions: Call a function in a loop until an exit key is pressed.

## CLI Features
 * hierarchical menus
 * command tab completion
 * command history
 * context sensitive help
 * command editing

## Examples

### ./examples/line/main.go
Matches the example code in the C version of the linenoise library.

### ./examples/cli/main.go
Implements an example of a heirarchical command line interface.

