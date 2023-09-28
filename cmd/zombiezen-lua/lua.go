// Copyright 2023 Ross Light
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the “Software”), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
//
// SPDX-License-Identifier: MIT

// zombiezen-lua is a standalone Lua interpreter.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"zombiezen.com/go/lua"
)

const programName = "zombiezen-lua"

func main() {
	programName := "zombiezen-lua"
	if len(os.Args) > 0 && os.Args[0] != "" {
		programName = filepath.Base(os.Args[0])
	}
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
	}
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	noEnv := flag.Bool("E", false, "ignore environment variables")
	flag.Parse()

	l := new(lua.State)
	if *noEnv {
		l.PushBoolean(true)
		l.RawSetField(lua.RegistryIndex, "LUA_NOENV")
	}
	if err := lua.OpenLibraries(l); err != nil {
		return err
	}
	var script int
	if len(os.Args) == 0 {
		script = -1
	} else if flag.NArg() == len(os.Args)-1 {
		script = 0
	} else {
		script = len(os.Args) - flag.NArg()
	}
	if err := createArgTable(l, os.Args, script); err != nil {
		return err
	}
	return doREPL(l)
}

func doREPL(l *lua.State) error {
	s := bufio.NewScanner(os.Stdin)
	for {
		if err := loadLine(l, s); errors.As(err, new(inputError)) {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		} else if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if err := doCall(l, 0, lua.MultipleReturns); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		print(l, "")
	}
}

func print(l *lua.State, errPrefix string) {
	n := l.Top()
	if n == 0 {
		return
	}
	if !l.CheckStack(20) {
		fmt.Fprintf(os.Stderr, "%stoo many results (%d) to print\n", errPrefix, n)
		return
	}
	if _, err := l.Global("print", 0); err != nil {
		fmt.Fprintf(os.Stderr, "%s%v\n", errPrefix, err)
		return
	}
	l.Insert(1)
	if err := l.Call(n, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "%serror calling 'print' (%v)\n", errPrefix, err)
		return
	}
}

func doCall(l *lua.State, nArgs, nResults int) error {
	base := l.Top() - nArgs
	l.PushClosure(0, msgHandler)
	l.Insert(base)
	// TODO(someday): Catch signals.
	err := l.Call(nArgs, nResults, base)
	l.Remove(base)
	return err
}

func msgHandler(l *lua.State) (int, error) {
	msg, ok := l.ToString(1)
	if !ok {
		if called, err := lua.CallMeta(l, 1, "__tostring"); called && err == nil && l.IsString(-1) {
			// Already pushed onto stack and it's a string.
			return 1, nil
		}
		msg = fmt.Sprintf("(error object is a %v value)", l.Type(1))
	}
	// TODO(soon): Append a standard traceback.
	l.PushString(msg)
	return 1, nil
}

func createArgTable(l *lua.State, args []string, script int) error {
	nArg := len(args) - (script + 1)
	l.CreateTable(nArg, script+1)
	for i, arg := range args {
		l.PushString(arg)
		l.RawSetIndex(-2, int64(i-script))
	}
	if err := l.SetGlobal("arg", 0); err != nil {
		return fmt.Errorf("create arg table: %v", err)
	}

	return nil
}

// loadLine reads a line and tries to compile it as an expression or statement.
func loadLine(l *lua.State, s *bufio.Scanner) error {
	l.SetTop(0)
	line, err := readLine(l, s, true)
	if err != nil {
		return err
	}
	if err := addReturn(l, line); err == nil {
		return nil
	}
	for {
		err := l.LoadString(line, "=stdin", "t")
		if err == nil {
			return nil
		}
		if !isIncomplete(err) {
			l.Pop(1)
			return err
		}
		newLine, err := readLine(l, s, false)
		if err != nil {
			return err
		}
		line += "\n" + newLine
	}
}

func readLine(l *lua.State, s *bufio.Scanner, firstLine bool) (string, error) {
	p, err := prompt(l, firstLine)
	if err != nil {
		return "", inputError{fmt.Errorf("read line: %v", err)}
	}
	os.Stdout.WriteString(p)
	if !s.Scan() {
		err := s.Err()
		if err == nil {
			err = io.EOF
		}
		return "", inputError{fmt.Errorf("read line: %w", err)}
	}
	line := s.Text()
	if firstLine && strings.HasPrefix(line, "=") {
		line = "return " + line
	}
	return line, nil
}

type inputError struct {
	err error
}

func (e inputError) Error() string {
	return e.err.Error()
}

func (e inputError) Unwrap() error {
	return e.err
}

func prompt(l *lua.State, firstLine bool) (string, error) {
	if firstLine {
		if tp, err := l.Global("_PROMPT", 0); err != nil {
			l.Pop(1)
			return "", err
		} else if tp == lua.TypeNil {
			l.Pop(1)
			return "> ", nil
		}
	} else {
		if tp, err := l.Global("_PROMPT2", 0); err != nil {
			l.Pop(1)
			return "", err
		} else if tp == lua.TypeNil {
			l.Pop(1)
			return ">> ", nil
		}
	}
	p, err := lua.ToString(l, -1)
	l.Pop(1)
	if err != nil {
		return "", fmt.Errorf("custom prompt: %v", err)
	}
	return p, nil
}

func addReturn(l *lua.State, line string) error {
	retLine := "return " + line + ";"
	if err := l.LoadString(retLine, "=stdin", "t"); err != nil {
		l.Pop(1)
		return err
	}
	return nil
}

func isIncomplete(err error) bool {
	if err == nil {
		return false
	}
	return lua.IsSyntax(err) && strings.Contains(err.Error(), "<eof>")
}
