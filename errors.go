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
