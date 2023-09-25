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

package lua

import (
	"errors"
	"fmt"
)

// #include "lua.h"
import "C"

type luaError struct {
	code C.int
	msg  string
}

func (l *State) newError(code C.int) error {
	e := &luaError{code: code}
	e.msg, _ = l.ToString(-1)
	return e
}

func (e *luaError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	switch e.code {
	case C.LUA_ERRRUN:
		return "runtime error"
	case C.LUA_ERRMEM:
		return "memory allocation error"
	case C.LUA_ERRERR:
		return "error while running message handler"
	case C.LUA_ERRSYNTAX:
		return "syntax error"
	case C.LUA_YIELD:
		return "coroutine yield"
	default:
		return "unknown error"
	}
}

func unwrapError(err error) error {
	var e *luaError
	if errors.As(err, &e) {
		return e
	}
	return err
}

// NewArgError returns a new error reporting a problem with argument arg
// of the Go function that called it,
// using a standard message that includes msg as a comment.
func NewArgError(l *State, arg int, msg string) error {
	ar := l.Stack(0).Info("n")
	if ar == nil {
		// No stack frame.
		return fmt.Errorf("%sbad argument #%d (%s)", Where(l, 1), arg, msg)
	}
	if ar.NameWhat == "method" {
		arg-- // do not count 'self'
		if arg == 0 {
			// Error is in the self argument itself.
			return fmt.Errorf("%scalling '%s' on bad self (%s)", Where(l, 1), ar.Name, msg)
		}
	}
	if ar.Name == "" {
		// TODO(someday): Find global function.
		ar.Name = "?"
	}
	return fmt.Errorf("%sbad argument #%d to '%s' (%s)", Where(l, 1), arg, ar.Name, msg)
}

// NewTypeError returns a new type error for the argument arg
// of the Go function that called it, using a standard message;
// tname is a "name" for the expected type.
func NewTypeError(l *State, arg int, tname string) error {
	var typeArg string
	if Metafield(l, arg, "__name") == TypeString {
		typeArg, _ = l.ToString(-1)
	} else if tp := l.Type(arg); tp == TypeLightUserdata {
		typeArg = "light userdata"
	} else {
		typeArg = tp.String()
	}
	return NewArgError(l, arg, fmt.Sprintf("%s expected, got %s", tname, typeArg))
}
