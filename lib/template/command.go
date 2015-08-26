package template

import (
	"bytes"
	"os"
	"os/exec"
	"text/template"
)

// Command returns a command with the following set:
//  - stdin set to the result of t.Execute(data)
//  - the environment set to the concatenation of
//    the current process' environment and env
// If an error is returned, it will be the return value of
// t.Execute(data).
func Command(t *template.Template, cmd string, data interface{}, env []string) (*exec.Cmd, error) {
	c := exec.Command(cmd)
	c.Env = append(os.Environ(), env...)
	var buf bytes.Buffer
	err := t.Execute(&buf, data)
	if err != nil {
		return nil, err
	}
	c.Stdin = &buf
	return c, nil
}

// CommandOut is like Command, except that it
// additionally sets the command's stdout and
// stderr to os.Stdout and os.Stdin.
func CommandOut(t *template.Template, cmd string, data interface{}, env []string) (*exec.Cmd, error) {
	c := exec.Command(cmd)
	c.Env = append(os.Environ(), env...)
	var buf bytes.Buffer
	err := t.Execute(&buf, data)
	if err != nil {
		return nil, err
	}
	c.Stdin = &buf
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c, nil
}

type Entry struct {
	// The variables to add to the subcommand's
	// environment.
	Env []string
	// The data to be passed to the template
	Data interface{}
}

// RunAll calls Command on each entry, and executes
// the resulting commands. All calls to Command take
// place before any command is executed, so any
// error in executing t will be encountered before
// any command is executed. All calls to Command
// and command executions take place in the order
// of the entries argument. If the returned error
// is non-nil, its type will be EntryError.
func RunAll(t *template.Template, cmd string, entries ...Entry) error {
	cmds := make([]*exec.Cmd, len(entries))
	for i, e := range entries {
		var err error
		cmds[i], err = Command(t, cmd, e.Data, e.Env)
		if err != nil {
			return EntryError{i, e, err}
		}
	}

	for i, c := range cmds {
		err := c.Run()
		if err != nil {
			return EntryError{i, entries[i], err}
		}
	}
	return nil
}

// RunAllOut is like RunAll, except that it calls
// CommandOut instead of Command.
func RunAllOut(t *template.Template, cmd string, entries ...Entry) error {
	cmds := make([]*exec.Cmd, len(entries))
	for i, e := range entries {
		var err error
		cmds[i], err = Command(t, cmd, e.Data, e.Env)
		if err != nil {
			return EntryError{i, e, err}
		}
	}

	for i, c := range cmds {
		err := c.Run()
		if err != nil {
			return EntryError{i, entries[i], err}
		}
	}
	return nil
}

// EntryError is the error type returned when
// there is an error executing the template or
// subcommand for a particular entry in the
// RunAll or RunAllOut functions.
type EntryError struct {
	Index int   // The entry's index in entries
	Entry Entry // The entry itself
	Err   error // The error encountered
}

func (e EntryError) Error() string { return e.Err.Error() }