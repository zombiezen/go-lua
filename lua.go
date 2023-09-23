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

// Package lua provides low-level bindings for Lua.
package lua

import (
	"errors"
	"fmt"
	"io"
	"runtime/cgo"
	"unsafe"
)

// #cgo unix CFLAGS: -DLUA_USE_POSIX
// #cgo unix LDFLAGS: -lm
// #include <stdlib.h>
// #include <stddef.h>
// #include <stdint.h>
// #include "lua.h"
// #include "lauxlib.h"
// #include "lualib.h"
//
// char *zombiezen_lua_readercb(lua_State *L, void *data, size_t *size);
// int zombiezen_lua_gocb(lua_State *L);
// int zombiezen_lua_gchandle(lua_State *L);
//
// int zombiezen_lua_callback(lua_State *L) {
//   int nresults = zombiezen_lua_gocb(L);
//   if (nresults < 0) {
//     lua_error(L);
//   }
//   return nresults;
// }
//
// void zombiezen_lua_pushstring(lua_State *L, _GoString_ s) {
//   lua_pushlstring(L, _GoStringPtr(s), _GoStringLen(s));
// }
//
// struct readStringData {
//   _GoString_ s;
//   int done;
// };
//
// static const char *readstring(lua_State *L, void *data, size_t *size) {
//   struct readStringData *myData = (struct readStringData*)(data);
//   if (myData->done) {
//     *size = 0;
//     return NULL;
//   }
//   *size = _GoStringLen(myData->s);
//   myData->done = 1;
//   return _GoStringPtr(myData->s);
// }
//
// static int loadstring(lua_State *L, _GoString_ s, const char* chunkname, const char* mode) {
//   struct readStringData myData = {s, 0};
//   return lua_load(L, readstring, &myData, chunkname, mode);
// }
//
// static int gettablecb(lua_State *L) {
//   lua_gettable(L, 1);
//   return 1;
// }
//
// static int gettable(lua_State *L, int index, int msgh, int *tp) {
//   index = lua_absindex(L, index);
//   msgh = msgh != 0 ? lua_absindex(L, msgh) : 0;
//   lua_pushcfunction(L, gettablecb);
//   lua_pushvalue(L, index);
//   lua_rotate(L, -3, -1);
//   int ret = lua_pcall(L, 2, 1, msgh);
//   if (tp != NULL) {
//     *tp = ret == LUA_OK ? lua_type(L, -1) : LUA_TNIL;
//   }
//   return ret;
// }
//
// static int settablecb(lua_State *L) {
//   lua_settable(L, 1);
//   return 0;
// }
//
// static int settable(lua_State *L, int index, int msgh) {
//   index = lua_absindex(L, index);
//   msgh = msgh != 0 ? lua_absindex(L, msgh) : 0;
//   lua_pushcfunction(L, settablecb);
//   lua_pushvalue(L, index);
//   lua_rotate(L, -4, -2);
//   return lua_pcall(L, 3, 0, msgh);
// }
//
// static void pushlightuserdata(lua_State *L, uint64_t p) {
//   lua_pushlightuserdata(L, (void *)p);
// }
import "C"

// RegistryIndex is a pseudo-index to the [registry],
// a predefined table that can be used by any Go or C code
// to store whatever Lua values it needs to store.
//
// [registry]: https://www.lua.org/manual/5.4/manual.html#4.3
const RegistryIndex int = C.LUA_REGISTRYINDEX

// Predefined keys in the registry.
const (
	// RegistryIndexMainThread is the index at which the registry has the main thread of the state.
	RegistryIndexMainThread int64 = C.LUA_RIDX_MAINTHREAD
	// RegistryIndexGlobals is the index at which the registry has the global environment.
	RegistryIndexGlobals int64 = C.LUA_RIDX_GLOBALS

	// LoadedTable is the key in the registry for the table of loaded modules.
	LoadedTable = C.LUA_LOADED_TABLE
	// PreloadTable is the key in the registry for the table of preloaded loaders.
	PreloadTable = C.LUA_PRELOAD_TABLE
)

// Type is an enumeration of Lua data types.
type Type C.int

// TypeNone is the value returned from [State.Type]
// for a non-valid but acceptable index.
const TypeNone Type = C.LUA_TNONE

// Value types.
const (
	TypeNil           Type = C.LUA_TNIL
	TypeBoolean       Type = C.LUA_TBOOLEAN
	TypeLightUserdata Type = C.LUA_TLIGHTUSERDATA
	TypeNumber        Type = C.LUA_TNUMBER
	TypeString        Type = C.LUA_TSTRING
	TypeTable         Type = C.LUA_TTABLE
	TypeFunction      Type = C.LUA_TFUNCTION
	TypeUserdata      Type = C.LUA_TUSERDATA
	TypeThread        Type = C.LUA_TTHREAD
)

// String returns the name of the type encoded by the value tp.
func (tp Type) String() string {
	switch tp {
	case TypeNone:
		return "no value"
	case TypeNil:
		return "nil"
	case TypeBoolean:
		return "boolean"
	case TypeLightUserdata, TypeUserdata:
		return "userdata"
	case TypeNumber:
		return "number"
	case TypeString:
		return "string"
	case TypeTable:
		return "table"
	case TypeFunction:
		return "function"
	case TypeThread:
		return "thread"
	default:
		return fmt.Sprintf("lua.Type(%d)", C.int(tp))
	}
}

// State represents a Lua execution thread.
// The zero value is a state with a single main thread,
// an empty stack, and an empty environment.
//
// The methods on State are primitive methods
// (i.e. the library functions that start with "lua_").
//
// Methods that take in stack indices have a notion of
// [valid and acceptable indices].
// If a method receives a stack index that is not within range,
// it will panic.
// Methods may also panic if there is insufficient stack space.
// Use [State.CheckStack]
// to ensure that the State has sufficient stack space before making calls,
// but note that any new State or called function
// will support pushing at least 20 values.
//
// [valid and acceptable indices]: https://www.lua.org/manual/5.4/manual.html#4.1.2
type State struct {
	ptr *C.lua_State
	top int
	cap int
}

// stateForCallback returns a new State for the given *lua_State.
// stateForCallback assumes that it is called
// before any other functions are called on the *lua_State.
func stateForCallback(ptr *C.lua_State) *State {
	l := &State{
		ptr: ptr,
		top: int(C.lua_gettop(ptr)),
	}
	l.cap = l.top + C.LUA_MINSTACK
	return l
}

func (l *State) init() {
	if l.ptr == nil {
		l.ptr = C.luaL_newstate()
		if l == nil {
			panic("could not allocate memory for new state")
		}
		C.lua_setwarnf(l.ptr, nil, nil)
		l.cap = C.LUA_MINSTACK
	}
}

// Close releases all resources associated with the state.
// Making further calls to the State will create a new execution environment.
func (l *State) Close() error {
	if l.ptr != nil {
		C.lua_close(l.ptr)
		*l = State{}
	}
	return nil
}

// AbsIndex converts the acceptable index idx
// into an equivalent absolute index
// (that is, one that does not depend on the stack size).
// AbsIndex panics if idx is not an acceptable index.
func (l *State) AbsIndex(idx int) int {
	switch {
	case isPseudo(idx):
		return idx
	case idx == 0 || idx < -l.top || idx > l.cap:
		panic("unacceptable index")
	case idx < 0:
		return l.top + idx + 1
	default:
		return idx
	}
}

func (l *State) isValidIndex(idx int) bool {
	if idx == goClosureUpvalueIndex {
		// Forbid users of the package from accessing the GoClosure upvalue.
		return false
	}
	if isPseudo(idx) {
		return true
	}
	if idx < 0 {
		idx = -idx
	}
	return 1 <= idx && idx <= l.top
}

func (l *State) isAcceptableIndex(idx int) bool {
	return l.isValidIndex(idx) || l.top <= idx && idx <= l.cap
}

func (l *State) checkElems(n int) {
	if l.top < n {
		panic("not enough elements in the stack")
	}
}

func (l *State) checkMessageHandler(msgHandler int) int {
	switch {
	case msgHandler == 0:
		return 0
	case isPseudo(msgHandler):
		panic("pseudo-indexed message handler")
	case 1 <= msgHandler && msgHandler <= l.top:
		return msgHandler
	case -l.top <= msgHandler && msgHandler <= -1:
		return l.top + msgHandler + 1
	default:
		panic("invalid message handler index")
	}
}

// Top returns the index of the top element in the stack.
// Because indices start at 1,
// this result is equal to the number of elements in the stack;
// in particular, 0 means an empty stack.
func (l *State) Top() int {
	return l.top
}

// SetTop accepts any index, or 0, and sets the stack top to this index.
// If the new top is greater than the old one,
// then the new elements are filled with nil.
// If idx is 0, then all stack elements are removed.
func (l *State) SetTop(idx int) {
	// lua_settop can raise errors, which will be undefined behavior,
	// but only if we mark stack slots as to-be-closed.
	// We have a simple solution: don't let the user do that.

	switch {
	case isPseudo(idx):
		panic("pseudo-index invalid for top")
	case idx == 0:
		if l.ptr != nil {
			C.lua_settop(l.ptr, 0)
			l.top = 0
		}
		return
	case idx < 0:
		idx += l.top + 1
		if idx < 0 {
			panic("stack underflow")
		}
	case idx > l.cap:
		panic("stack overflow")
	}
	l.init()

	C.lua_settop(l.ptr, C.int(idx))
	l.top = idx
}

// Pop pops n elements from the stack.
func (l *State) Pop(n int) {
	l.SetTop(-n - 1)
}

// PushValue pushes a copy of the element at the given index onto the stack.
func (l *State) PushValue(idx int) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushvalue(l.ptr, C.int(idx))
	l.top++
}

// Rotate rotates the stack elements
// between the valid index idx and the top of the stack.
// The elements are rotated n positions in the direction of the top, for a positive n,
// or -n positions in the direction of the bottom, for a negative n.
// If the absolute value of n is greater than the size of the slice being rotated,
// Rotate panics.
// This function cannot be called with a pseudo-index,
// because a pseudo-index is not an actual stack position.
func (l *State) Rotate(idx, n int) {
	l.init()
	if !l.isValidIndex(idx) || isPseudo(idx) {
		panic("invalid index")
	}
	idx = l.AbsIndex(idx)
	absN := n
	if n < 0 {
		absN = -n
	}
	if absN > l.top-idx+1 {
		panic("invalid rotation")
	}
	C.lua_rotate(l.ptr, C.int(idx), C.int(n))
}

// Remove removes the element at the given valid index,
// shifting down the elements above this index to fill the gap.
// This function cannot be called with a pseudo-index,
// because a pseudo-index is not an actual stack position.
func (l *State) Remove(idx int) {
	l.Rotate(idx, -1)
	l.Pop(1)
}

// Copy copies the element at index fromIdx into the valid index toIdx,
// replacing the value at that position.
// Values at other positions are not affected.
func (l *State) Copy(fromIdx, toIdx int) {
	l.init()
	if !l.isAcceptableIndex(fromIdx) || !l.isAcceptableIndex(toIdx) {
		panic("unacceptable index")
	}
	C.lua_copy(l.ptr, C.int(fromIdx), C.int(toIdx))
}

// Replace moves the top element into the given valid index without shifting any element
// (therefore replacing the value at that given index),
// and then pops the top element.
func (l *State) Replace(idx int) {
	l.Copy(-1, idx)
	l.Pop(1)
}

// CheckStack ensures that the stack has space for at least n extra elements,
// that is, that you can safely push up to n values into it.
// It returns false if it cannot fulfill the request,
// either because it would cause the stack to be greater than a fixed maximum size
// (typically at least several thousand elements)
// or because it cannot allocate memory for the extra space.
// This function never shrinks the stack;
// if the stack already has space for the extra elements, it is left unchanged.
func (l *State) CheckStack(n int) bool {
	if l.top+n <= l.cap {
		return true
	}
	l.init()
	ok := C.lua_checkstack(l.ptr, C.int(n)) != 0
	if ok {
		l.cap = max(l.cap, l.top+n)
	}
	return ok
}

// IsNumber reports if the value at the given index is a number
// or a string convertible to a number.
func (l *State) IsNumber(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_isnumber(l.ptr, C.int(idx)) != 0
}

// IsString reports if the value at the given index is a string
// or a number (which is always convertible to a string).
func (l *State) IsString(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_isstring(l.ptr, C.int(idx)) != 0
}

// IsNativeFunction reports if the value at the given index is a C or Go function.
func (l *State) IsNativeFunction(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_iscfunction(l.ptr, C.int(idx)) != 0
}

// IsInteger reports if the value at the given index is an integer
// (that is, the value is a number and is represented as an integer).
func (l *State) IsInteger(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_isinteger(l.ptr, C.int(idx)) != 0
}

// IsUserdata reports if the value at the given index is a userdata (either full or light).
func (l *State) IsUserdata(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_isuserdata(l.ptr, C.int(idx)) != 0
}

// Type returns the type of the value in the given valid index,
// or [TypeNone] for a non-valid but acceptable index.
func (l *State) Type(idx int) Type {
	if l.ptr == nil {
		return TypeNone
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return Type(C.lua_type(l.ptr, C.int(idx)))
}

// IsFunction reports if the value at the given index is a function (any of C, Go, or Lua).
func (l *State) IsFunction(idx int) bool {
	return l.Type(idx) == TypeFunction
}

// IsTable reports if the value at the given index is a table.
func (l *State) IsTable(idx int) bool {
	return l.Type(idx) == TypeTable
}

// IsNil reports if the value at the given index is nil.
func (l *State) IsNil(idx int) bool {
	return l.Type(idx) == TypeNil
}

// IsBoolean reports if the value at the given index is a boolean.
func (l *State) IsBoolean(idx int) bool {
	return l.Type(idx) == TypeBoolean
}

// IsThread reports if the value at the given index is a thread.
func (l *State) IsThread(idx int) bool {
	return l.Type(idx) == TypeThread
}

// IsNone reports if the index is not valid.
func (l *State) IsNone(idx int) bool {
	return l.Type(idx) == TypeNone
}

// IsNoneOrNil reports if the index is not valid or the value at this index is nil.
func (l *State) IsNoneOrNil(idx int) bool {
	tp := l.Type(idx)
	return tp == TypeNone || tp == TypeNil
}

// ToNumber converts the Lua value at the given index to a floating point number.
// The Lua value must be a number or a [string convertible to a number];
// otherwise, ToNumber returns (0, false).
// ok is true if the operation succeeded.
//
// [string convertible to a number]: https://www.lua.org/manual/5.4/manual.html#3.4.3
func (l *State) ToNumber(idx int) (n float64, ok bool) {
	if l.ptr == nil {
		return 0, false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	var isNum C.int
	n = float64(C.lua_tonumberx(l.ptr, C.int(idx), &isNum))
	return n, isNum != 0
}

// ToInteger converts the Lua value at the given index to a signed 64-bit integer.
// The Lua value must be an integer, a number, or a [string convertible to an integer];
// otherwise, ToInteger returns (0, false).
// ok is true if the operation succeeded.
//
// [string convertible to an integer]: https://www.lua.org/manual/5.4/manual.html#3.4.3
func (l *State) ToInteger(idx int) (n int64, ok bool) {
	if l.ptr == nil {
		return 0, false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	var isNum C.int
	n = int64(C.lua_tointegerx(l.ptr, C.int(idx), &isNum))
	return n, isNum != 0
}

// ToBoolean converts the Lua value at the given index to a boolean value.
// Like all tests in Lua,
// ToBoolean returns true for any Lua value different from false and nil;
// otherwise it returns false.
func (l *State) ToBoolean(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_toboolean(l.ptr, C.int(idx)) != 0
}

// ToString converts the Lua value at the given index to a Go string.
// The Lua value must be a string or a number; otherwise, the function returns ("", false).
// If the value is a number, then ToString also changes the actual value in the stack to a string.
// (This change confuses [State.Next]
// when ToString is applied to keys during a table traversal.)
func (l *State) ToString(idx int) (s string, ok bool) {
	if l.ptr == nil {
		return "", false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	var len C.size_t
	ptr := C.lua_tolstring(l.ptr, C.int(idx), &len)
	if ptr == nil {
		return "", false
	}
	return C.GoStringN(ptr, C.int(len)), true
}

// RawLen returns the raw "length" of the value at the given index:
// for strings, this is the string length;
// for tables, this is the result of the length operator ('#') with no metamethods;
// for userdata, this is the size of the block of memory allocated for the userdata.
// For other values, RawLen returns 0.
func (l *State) RawLen(idx int) uint64 {
	if l.ptr == nil {
		return 0
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return uint64(C.lua_rawlen(l.ptr, C.int(idx)))
}

// ToGoValue converts the Lua value at the given index to a Go value.
// The Lua value must be a userdata previously created by [State.PushGoValue];
// otherwise the function returns nil.
func (l *State) ToGoValue(idx int) any {
	if l.ptr == nil {
		return nil
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	handlePtr := l.testHandle(idx)
	if handlePtr == nil || *handlePtr == 0 {
		return nil
	}
	return handlePtr.Value()
}

// ToPointer converts the value at the given index to a generic pointer
// and returns its numeric address.
// The value can be a userdata, a table, a thread, a string, or a function;
// otherwise, ToPointer returns 0.
// Different objects will give different addresses.
// Typically this function is used only for hashing and debug information.
func (l *State) ToPointer(idx int) uintptr {
	if l.ptr == nil {
		return 0
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return uintptr(C.lua_topointer(l.ptr, C.int(idx)))
}

// RawEqual reports whether the two values in the given indices
// are primitively equal (that is, equal without calling the __eq metamethod).
func (l *State) RawEqual(idx1, idx2 int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx1) || !l.isAcceptableIndex(idx2) {
		panic("unacceptable index")
	}
	return C.lua_rawequal(l.ptr, C.int(idx1), C.int(idx2)) != 0
}

// PushNil pushes a nil value onto the stack.
func (l *State) PushNil() {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushnil(l.ptr)
	l.top++
}

// PushNumber pushes a floating point number onto the stack.
func (l *State) PushNumber(n float64) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushnumber(l.ptr, C.lua_Number(n))
	l.top++
}

// PushInteger pushes an integer onto the stack.
func (l *State) PushInteger(n int64) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushinteger(l.ptr, C.lua_Integer(n))
	l.top++
}

// PushString pushes a string onto the stack.
func (l *State) PushString(s string) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.zombiezen_lua_pushstring(l.ptr, s)
	l.top++
}

// PushBoolean pushes a boolean onto the stack.
func (l *State) PushBoolean(b bool) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	i := C.int(0)
	if b {
		i = 1
	}
	C.lua_pushboolean(l.ptr, i)
	l.top++
}

// PushLightUserdata pushes a light userdata onto the stack.
//
// Userdata represent C or Go values in Lua.
// A light userdata represents a pointer.
// It is a value (like a number): you do not create it, it has no individual metatable,
// and it is not collected (as it was never created).
// A light userdata is equal to "any" light userdata with the same address.
func (l *State) PushLightUserdata(p uintptr) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.pushlightuserdata(l.ptr, C.uint64_t(p))
	l.top++
}

// PushGoValue pushes a Go userdata onto the stack.
// If v is nil, a nil will be pushed instead.
// The value can be retrieved later with [State.ToGoValue].
//
// PushGoValue creates a userdata with a metatable
// that has a __gc method to clean up the reference to the Go value
// when it is garbage-collected by Lua.
// If the metatable is tampered with, then the Go value can be leaked.
// The metatable has the __metatable field set to false,
// so it cannot be accessed through Lua's getmetatable function in the basic library,
// but it is still accessible through the Go/C API and debug interfaces.
func (l *State) PushGoValue(v any) {
	if v == nil {
		l.PushNil()
	} else {
		l.init()
		l.pushHandle(cgo.NewHandle(v))
	}
}

// A Function is a callback for Lua function implemented in Go.
// A Go function receives its arguments from Lua in its stack in direct order
// (the first argument is pushed first).
// So, when the function starts,
// [State.Top] returns the number of arguments received by the function.
// The first argument (if any) is at index 1 and its last argument is at index [State.Top].
// To return values to Lua, a Go function just pushes them onto the stack,
// in direct order (the first result is pushed first),
// and returns in Go the number of results.
// Any other value in the stack below the results will be properly discarded by Lua.
// Like a Lua function, a Go function called by Lua can also return many results.
// To raise an error, return a Go error
// and the string result of its Error() method will be used as the error object.
type Function func(*State) (int, error)

func (f Function) pcall(l *State) (nResults int, err error) {
	defer func() {
		if v := recover(); v != nil {
			nResults = 0
			switch v := v.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = fmt.Errorf("%v", v)
			}
		}
	}()
	return f(l)
}

// PushClosure pushes a Go closure onto the stack.
// n is how many upvalues this function will have,
// popped off the top of the stack.
// (When there are multiple upvalues, the first value is pushed first.)
// If n is negative or greater than 254, then PushClosure panics.
//
// Under the hood, PushClosure uses the first Lua upvalue
// to store a reference to the Go function.
// [UpvalueIndex] already compensates for this,
// so the first upvalue you push with PushClosure
// can be accessed with UpvalueIndex(1).
// As such, this implementation detail is largely invisible
// except in debug interfaces.
// No assumptions should be made about the content of the first upvalue,
// as it is subject to change,
// but it is guaranteed that PushClosure will use exactly one upvalue.
func (l *State) PushClosure(n int, f Function) {
	if n < 0 || n > 254 {
		panic("invalid upvalue count")
	}
	l.init()
	l.checkElems(n)
	// pushHandle handles checking the stack.
	l.pushHandle(cgo.NewHandle(f))
	l.Rotate(-(n + 1), 1)
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.zombiezen_lua_callback), 1+C.int(n))
	// lua_pushcclosure pops n+1, but pushes 1.
	l.top -= n
}

// Global pushes onto the stack the value of the global with the given name,
// returning the type of that value.
//
// As in Lua, this function may trigger a metamethod on the globals table
// for the "index" event.
// If there is any error, Global catches it,
// pushes a single value on the stack (the error object),
// and returns an error with [TypeNil].
//
// If msgHandler is 0,
// then the error object returned on the stack is exactly the original error object.
// Otherwise, msgHandler is the stack index of a message handler.
// (This index cannot be a pseudo-index.)
// In case of runtime errors, this handler will be called with the error object
// and its return value will be the object returned on the stack by Table.
// Typically, the message handler is used to add more debug information to the error object,
// such as a stack traceback.
// Such information cannot be gathered after the return of Table,
// since by then the stack has unwound.
func (l *State) Global(name string, msgHandler int) (Type, error) {
	l.init()
	msgHandler = l.checkMessageHandler(msgHandler)
	l.RawIndex(RegistryIndex, RegistryIndexGlobals)
	tp, err := l.Field(-1, name, msgHandler)
	l.Remove(-2) // remove the globals table
	return tp, err
}

// Table pushes onto the stack the value t[k],
// where t is the value at the given index
// and k is the value on the top of the stack.
// Returns the type of the pushed value.
//
// This function pops the key from the stack,
// pushing the resulting value in its place.
//
// As in Lua, this function may trigger a metamethod for the "index" event.
// If there is any error, Table catches it,
// pushes a single value on the stack (the error object),
// and returns an error with [TypeNil].
// Table always removes the key from the stack.
//
// If msgHandler is 0,
// then the error object returned on the stack is exactly the original error object.
// Otherwise, msgHandler is the stack index of a message handler.
// (This index cannot be a pseudo-index.)
// In case of runtime errors, this handler will be called with the error object
// and its return value will be the object returned on the stack by Table.
// Typically, the message handler is used to add more debug information to the error object,
// such as a stack traceback.
// Such information cannot be gathered after the return of Table,
// since by then the stack has unwound.
func (l *State) Table(idx, msgHandler int) (Type, error) {
	l.checkElems(1)
	if !l.CheckStack(2) { // gettable needs 2 additional stack slots
		panic("stack overflow")
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	msgHandler = l.checkMessageHandler(msgHandler)
	var tp C.int
	ret := C.gettable(l.ptr, C.int(idx), C.int(msgHandler), &tp)
	if ret != C.LUA_OK {
		return TypeNil, fmt.Errorf("lua: table lookup: %w", l.newError(ret))
	}
	return Type(tp), nil
}

// Field pushes onto the stack the value t[k],
// where t is the value at the given index.
// See [State.Table] for further information.
func (l *State) Field(idx int, k string, msgHandler int) (Type, error) {
	l.init()
	if !l.CheckStack(3) { // gettable needs 2 additional stack slots
		panic("stack overflow")
	}
	idx = l.AbsIndex(idx)
	msgHandler = l.checkMessageHandler(msgHandler)
	l.PushString(k)
	var tp C.int
	ret := C.gettable(l.ptr, C.int(idx), C.int(msgHandler), &tp)
	if ret != C.LUA_OK {
		return TypeNil, fmt.Errorf("lua: get field %q: %w", k, l.newError(ret))
	}
	return Type(tp), nil
}

// RawGet pushes onto the stack t[k],
// where t is the value at the given index
// and k is the value on the top of the stack.
// This function pops the key from the stack,
// pushing the resulting value in its place.
//
// RawGet does a raw access (i.e. without metamethods).
// The value at idx must be a table.
func (l *State) RawGet(idx int) Type {
	l.checkElems(1)
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	tp := Type(C.lua_rawget(l.ptr, C.int(idx)))
	return tp
}

// RawIndex pushes onto the stack the value t[n],
// where t is the table at the given index.
// The access is raw, that is, it does not use the __index metavalue.
// Returns the type of the pushed value.
func (l *State) RawIndex(idx int, n int64) Type {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	tp := Type(C.lua_rawgeti(l.ptr, C.int(idx), C.lua_Integer(n)))
	l.top++
	return tp
}

// RawField pushes onto the stack t[k],
// where t is the value at the given index.
//
// RawField does a raw access (i.e. without metamethods).
// The value at idx must be a table.
func (l *State) RawField(idx int, k string) Type {
	idx = l.AbsIndex(idx)
	l.PushString(k)
	return l.RawGet(idx)
}

// CreateTable creates a new empty table and pushes it onto the stack.
// nArr is a hint for how many elements the table will have as a sequence;
// nRec is a hint for how many other elements the table will have.
// Lua may use these hints to preallocate memory for the new table.
func (l *State) CreateTable(nArr, nRec int) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_createtable(l.ptr, C.int(nArr), C.int(nRec))
	l.top++
}

// NewUserdataUV creates and pushes on the stack a new full userdata,
// with nUValue associated Lua values, called user values.
// These values can be accessed or modified
// using [State.UserValue] and [State.SetUserValue] respectively.
func (l *State) NewUserdataUV(nUValue int) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_newuserdatauv(l.ptr, 0, C.int(nUValue))
	l.top++
}

// Metatable reports whether the value at the given index has a metatable
// and if so, pushes that metatable onto the stack.
func (l *State) Metatable(idx int) bool {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return l.metatable(idx)
}

func (l *State) metatable(idx int) bool {
	ok := C.lua_getmetatable(l.ptr, C.int(idx)) != 0
	if ok {
		l.top++
	}
	return ok
}

// UserValue pushes onto the stack the n-th user value
// associated with the full userdata at the given index
// and returns the type of the pushed value.
// If the userdata does not have that value, pushes nil and returns [TypeNone].
// (As with other Lua APIs, the first user value is n=1.)
func (l *State) UserValue(idx int, n int) Type {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	tp := TypeNone
	if n < 1 {
		C.lua_pushnil(l.ptr)
	} else {
		tp = Type(C.lua_getiuservalue(l.ptr, C.int(idx), C.int(n)))
	}
	l.top++
	return tp
}

// SetGlobal pops a value from the stack
// and sets it as the new value of the global with the given name.
//
// As in Lua, this function may trigger a metamethod on the globals table
// for the "newindex" event.
// If there is any error, SetGlobal catches it,
// pushes a single value on the stack (the error object),
// and returns an error.
// SetGlobal always removes the value from the stack.
//
// If msgHandler is 0,
// then the error object returned on the stack is exactly the original error object.
// Otherwise, msgHandler is the stack index of a message handler.
// (This index cannot be a pseudo-index.)
// In case of runtime errors, this handler will be called with the error object
// and its return value will be the object returned on the stack by SetGlobal.
// Typically, the message handler is used to add more debug information to the error object,
// such as a stack traceback.
// Such information cannot be gathered after the return of SetGlobal,
// since by then the stack has unwound.
func (l *State) SetGlobal(name string, msgHandler int) error {
	l.checkElems(1)
	if msgHandler != 0 {
		msgHandler = l.AbsIndex(msgHandler)
	}
	l.RawIndex(RegistryIndex, RegistryIndexGlobals)
	l.Rotate(-2, 1) // swap globals table with value
	err := l.SetField(-2, name, msgHandler)
	l.Pop(1) // remove the globals table
	return err
}

// SetTable does the equivalent to t[k] = v,
// where t is the value at the given index,
// v is the value on the top of the stack,
// and k is the value just below the top.
// This function pops both the key and the value from the stack.
//
// As in Lua, this function may trigger a metamethod for the "newindex" event.
// If there is any error, SetTable catches it,
// pushes a single value on the stack (the error object),
// and returns an error.
// SetTable always removes the key and value from the stack.
//
// If msgHandler is 0,
// then the error object returned on the stack is exactly the original error object.
// Otherwise, msgHandler is the stack index of a message handler.
// (This index cannot be a pseudo-index.)
// In case of runtime errors, this handler will be called with the error object
// and its return value will be the object returned on the stack by SetTable.
// Typically, the message handler is used to add more debug information to the error object,
// such as a stack traceback.
// Such information cannot be gathered after the return of SetTable,
// since by then the stack has unwound.
func (l *State) SetTable(idx, msgHandler int) error {
	l.checkElems(2)
	if !l.CheckStack(2) { // settable needs 2 additional stack slots
		panic("stack overflow")
	}
	if !l.isAcceptableIndex(idx) || msgHandler != 0 && !l.isAcceptableIndex(msgHandler) {
		panic("unacceptable index")
	}
	ret := C.settable(l.ptr, C.int(idx), C.int(msgHandler))
	if ret != C.LUA_OK {
		l.top--
		return fmt.Errorf("lua: set table field: %w", l.newError(ret))
	}
	l.top -= 2
	return nil
}

// SetField does the equivalent to t[k] = v,
// where t is the value at the given index,
// v is the value on the top of the stack,
// and k is the given string.
// This function pops the value from the stack.
// See [State.SetTable] for more information.
func (l *State) SetField(idx int, k string, msgHandler int) error {
	l.checkElems(1)
	if !l.CheckStack(3) { // settable needs 2 additional stack slots
		panic("stack overflow")
	}

	idx = l.AbsIndex(idx)
	if msgHandler != 0 {
		msgHandler = l.AbsIndex(msgHandler)
	}

	l.PushString(k)
	l.Rotate(-2, 1)
	ret := C.settable(l.ptr, C.int(idx), C.int(msgHandler))
	if ret != C.LUA_OK {
		l.top--
		return fmt.Errorf("lua: set field %q: %w", k, l.newError(ret))
	}
	l.top -= 2
	return nil
}

// RawSet does the equivalent to t[k] = v,
// where t is the value at the given index,
// v is the value on the top of the stack,
// and k is the value just below the top.
// This function pops both the key and the value from the stack.
func (l *State) RawSet(idx int) {
	l.checkElems(2)
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	C.lua_rawset(l.ptr, C.int(idx))
	l.top -= 2
}

// RawSetIndex does the equivalent of t[n] = v,
// where t is the table at the given index
// and v is the value on the top of the stack.
// This function pops the value from the stack.
// The assignment is raw, that is, it does not use the __newindex metavalue.
func (l *State) RawSetIndex(idx int, n int64) {
	l.checkElems(1)
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	C.lua_rawseti(l.ptr, C.int(idx), C.lua_Integer(n))
	l.top--
}

// RawSetField does the equivalent to t[k] = v,
// where t is the value at the given index
// and v is the value on the top of the stack.
// This function pops the value from the stack.
func (l *State) RawSetField(idx int, k string) {
	idx = l.AbsIndex(idx)
	l.PushString(k)
	l.Rotate(-2, 1)
	l.RawSet(idx)
}

// SetMetatable pops a table or nil from the stack
// and sets that value as the new metatable for the value at the given index.
// (nil means no metatable.)
func (l *State) SetMetatable(objIndex int) {
	l.checkElems(1)
	if !l.isAcceptableIndex(objIndex) {
		panic("unacceptable index")
	}
	C.lua_setmetatable(l.ptr, C.int(objIndex))
	l.top--
}

// SetUserValue pops a value from the stack
// and sets it as the new n-th user value
// associated to the full userdata at the given index,
// reporting if the userdata has that value.
// (As with other Lua APIs, the first user value is n=1.)
func (l *State) SetUserValue(idx int, n int) bool {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	if n < 1 {
		l.Pop(1)
		return false
	}
	ok := C.lua_setiuservalue(l.ptr, C.int(idx), C.int(n)) != 0
	l.top--
	return ok
}

// Call calls a function (or callable object) in protected mode.
//
// To do a call you must use the following protocol:
// first, the function to be called is pushed onto the stack;
// then, the arguments to the call are pushed in direct order;
// that is, the first argument is pushed first.
// Finally you call Call;
// nArgs is the number of arguments that you pushed onto the stack.
// When the function returns,
// all arguments and the function value are popped
// and the call results are pushed onto the stack.
// The number of results is adjusted to nResults,
// unless nResults is [MultipleReturns].
// In this case, all results from the function are pushed;
// Lua takes care that the returned values fit into the stack space,
// but it does not ensure any extra space in the stack.
// The function results are pushed onto the stack in direct order
// (the first result is pushed first),
// so that after the call the last result is on the top of the stack.
//
// If there is any error, Call catches it,
// pushes a single value on the stack (the error object),
// and returns an error.
// Call always removes the function and its arguments from the stack.
//
// If msgHandler is 0,
// then the error object returned on the stack is exactly the original error object.
// Otherwise, msgHandler is the stack index of a message handler.
// (This index cannot be a pseudo-index.)
// In case of runtime errors, this handler will be called with the error object
// and its return value will be the object returned on the stack by Call.
// Typically, the message handler is used to add more debug information to the error object,
// such as a stack traceback.
// Such information cannot be gathered after the return of Call,
// since by then the stack has unwound.
func (l *State) Call(nArgs, nResults, msgHandler int) error {
	if nArgs < 0 {
		panic("negative arguments")
	}
	toPop := 1 + nArgs
	l.checkElems(toPop)
	newTop := -1
	if nResults != MultipleReturns {
		if nResults < 0 {
			panic("negative results")
		}
		newTop = l.top - toPop + nResults
		if newTop > l.cap {
			panic("stack overflow")
		}
	}
	msgHandler = l.checkMessageHandler(msgHandler)

	ret := C.lua_pcallk(l.ptr, C.int(nArgs), C.int(nResults), C.int(msgHandler), 0, nil)
	if ret != C.LUA_OK {
		l.top -= toPop - 1
		return fmt.Errorf("lua: call: %w", l.newError(ret))
	}
	if newTop >= 0 {
		l.top = newTop
	} else {
		l.top = int(C.lua_gettop(l.ptr))
		l.cap = max(l.cap, l.top)
	}
	return nil
}

// MultipleReturns is the option for multiple returns in [State.Call].
const MultipleReturns int = C.LUA_MULTRET

// Load loads a Lua chunk without running it.
// If there are no errors,
// Load pushes the compiled chunk as a Lua function on top of the stack.
// Otherwise, it pushes an error message.
//
// The chunkName argument gives a name to the chunk,
// which is used for error messages and in [debug information].
//
// The string mode controls whether the chunk can be text or binary
// (that is, a precompiled chunk).
// It may be the string "b" (only binary chunks),
// "t" (only text chunks),
// or "bt" (both binary and text).
//
// [debug information]: https://www.lua.org/manual/5.4/manual.html#4.7
func (l *State) Load(r io.Reader, chunkName string, mode string) error {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}

	modeC, err := loadMode(mode)
	if err != nil {
		l.PushString(err.Error())
		return fmt.Errorf("lua: load %s: %v", formatChunkName(chunkName), err)
	}

	rr := newReader(r)
	defer rr.free()
	handle := cgo.NewHandle(rr)
	defer handle.Delete()

	chunkNameC := C.CString(chunkName)
	defer C.free(unsafe.Pointer(chunkNameC))

	ret := C.lua_load(l.ptr, C.lua_Reader(C.zombiezen_lua_readercb), unsafe.Pointer(&handle), chunkNameC, modeC)
	l.top++
	if ret != C.LUA_OK {
		return fmt.Errorf("lua: load %s: %w", formatChunkName(chunkName), l.newError(ret))
	}
	return nil
}

// LoadString loads a Lua chunk from a string without running it.
// It behaves the same as [State.Load],
// but takes in a string instead of an [io.Reader].
func (l *State) LoadString(s string, chunkName string, mode string) error {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}

	modeC, err := loadMode(mode)
	if err != nil {
		l.PushString(err.Error())
		return fmt.Errorf("lua: load %s: %v", formatChunkName(chunkName), err)
	}

	chunkNameC := C.CString(chunkName)
	defer C.free(unsafe.Pointer(chunkNameC))

	ret := C.loadstring(l.ptr, s, chunkNameC, modeC)
	l.top++
	if ret != C.LUA_OK {
		return fmt.Errorf("lua: load %s: %w", formatChunkName(chunkName), l.newError(ret))
	}
	return nil
}

func formatChunkName(chunkName string) string {
	if len(chunkName) == 0 || (chunkName[0] != '@' && chunkName[0] != '=') {
		return "(string)"
	}
	return chunkName[1:]
}

func loadMode(mode string) (*C.char, error) {
	const modeCStrings = "bt\x00t\x00b\x00"
	switch mode {
	case "bt":
		return (*C.char)(unsafe.Pointer(unsafe.StringData(modeCStrings))), nil
	case "t":
		return (*C.char)(unsafe.Pointer(unsafe.StringData(modeCStrings[3:]))), nil
	case "b":
		return (*C.char)(unsafe.Pointer(unsafe.StringData(modeCStrings[5:]))), nil
	default:
		return nil, fmt.Errorf("unknown load mode %q", mode)
	}
}

// Next pops a key from the stack,
// and pushes a key–value pair from the table at the given index,
// the "next" pair after the given key.
// If there are no more elements in the table,
// then Next returns false and pushes nothing.
//
// While traversing a table,
// avoid calling [State.ToString] directly on a key,
// unless you know that the key is actually a string.
// Recall that [State.ToString] may change the value at the given index;
// this confuses the next call to Next.
//
// This behavior of this function is undefined if the given key
// is neither nil nor present in the table.
// See function [next] for the caveats of modifying the table during its traversal.
//
// [next]: https://www.lua.org/manual/5.4/manual.html#pdf-next
func (l *State) Next(idx int) bool {
	l.checkElems(1)
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	ok := C.lua_next(l.ptr, C.int(idx)) != 0
	if ok {
		l.top++
	} else {
		l.top--
	}
	return ok
}

// Standard library names.
const (
	GName = C.LUA_GNAME

	CoroutineLibraryName = C.LUA_COLIBNAME
	TableLibraryName     = C.LUA_TABLIBNAME
	IOLibraryName        = C.LUA_IOLIBNAME
	OSLibraryName        = C.LUA_OSLIBNAME
	StringLibraryName    = C.LUA_STRLIBNAME
	UTF8LibraryName      = C.LUA_UTF8LIBNAME
	MathLibraryName      = C.LUA_MATHLIBNAME
	DebugLibraryName     = C.LUA_DBLIBNAME
	PackageLibraryName   = C.LUA_LOADLIBNAME
)

// PushOpenBase pushes a function onto the stack
// that loads the basic library.
// The print function will write to the given writer.
func (l *State) PushOpenBase(out io.Writer) {
	l.PushClosure(0, func(l *State) (int, error) {
		nArgs := l.Top()
		C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_base), 0)
		l.top++
		for i := 1; i <= nArgs; i++ {
			l.PushValue(i)
		}
		if err := l.Call(nArgs, 1, 0); err != nil {
			return 0, err
		}

		l.PushClosure(0, func(l *State) (int, error) {
			n := l.Top()
			for i := 1; i <= n; i++ {
				s, err := ToString(l, i)
				if err != nil {
					return 0, err
				}
				if i > 1 {
					io.WriteString(out, "\t")
				}
				io.WriteString(out, s)
			}
			io.WriteString(out, "\n")
			return 0, nil
		})
		l.RawSetField(-2, "print")
		return 1, nil
	})
}

func (l *State) PushOpenString() {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_string), 0)
	l.top++
}

func (l *State) PushOpenUTF8() {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_utf8), 0)
	l.top++
}

func (l *State) PushOpenTable() {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_table), 0)
	l.top++
}

func (l *State) PushOpenCoroutine() {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_coroutine), 0)
	l.top++
}

func (l *State) PushOpenMath() {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_math), 0)
	l.top++
}

func (l *State) PushOpenDebug() {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_debug), 0)
	l.top++
}

const readerBufferSize = 4096

type reader struct {
	r   io.Reader
	buf *C.char
}

func newReader(r io.Reader) *reader {
	return &reader{
		r:   r,
		buf: (*C.char)(C.calloc(readerBufferSize, C.size_t(unsafe.Sizeof(C.char(0))))),
	}
}

func (r *reader) free() {
	if r.buf != nil {
		C.free(unsafe.Pointer(r.buf))
		r.buf = nil
	}
}

const handleMetatableName = "runtime/cgo.Handle"

func (l *State) pushHandle(handle cgo.Handle) {
	if !l.CheckStack(3) {
		panic("stack overflow")
	}
	ptr := (*cgo.Handle)(C.lua_newuserdatauv(l.ptr, C.size_t(unsafe.Sizeof(cgo.Handle(0))), 0))
	*ptr = handle
	l.top++
	if NewMetatable(l, handleMetatableName) {
		C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.zombiezen_lua_gchandle), 0)
		l.top++
		l.RawSetField(-2, "__gc") // metatable.__gc = zombiezen_lua_gchandle
		// Prevent access of metatable from Lua.
		// The basic library's getmetatable function obeys this metafield.
		l.PushBoolean(false)
		l.RawSetField(-2, "__metatable") // metatable.__metatable = false
	}
	l.SetMetatable(-2)
}

func (l *State) testHandle(idx int) *cgo.Handle {
	p := C.lua_touserdata(l.ptr, C.int(idx))
	if p == nil {
		return nil
	}
	if !l.metatable(idx) {
		return nil
	}
	tp := Metatable(l, handleMetatableName)
	// Since we lazily create the cgo.Handle metatable,
	// we only want this to succeed if the metatable is not nil.
	// Otherwise, this is an unknown pointer we would be dereferencing.
	ok := tp == TypeTable && l.RawEqual(-1, -2)
	l.Pop(2)
	if !ok {
		return nil
	}
	return (*cgo.Handle)(p)
}

func isPseudo(i int) bool {
	return i <= RegistryIndex
}

const goClosureUpvalueIndex = C.LUA_REGISTRYINDEX - 1

// UpvalueIndex returns the pseudo-index that represents the i-th upvalue
// of the running function.
// If i is outside the range [1, 255], UpvalueIndex panics.
func UpvalueIndex(i int) int {
	if i < 1 || i > 255 {
		panic("invalid upvalue index")
	}
	return C.LUA_REGISTRYINDEX - (i + 1)
}
