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

// #include "lua.h"
import "C"
import "errors"

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
