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

package lua54

import (
	"errors"
	"fmt"
	"io"
	"runtime/cgo"
	"strings"
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
// int zombiezen_lua_writercb(lua_State *L, const void *p, size_t size, void *ud);
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
// const char *zombiezen_lua_reader(lua_State *L, void *data, size_t *size) {
//   const char *p = zombiezen_lua_readercb(L, data, size);
//   if (p == NULL) {
//     lua_error(L);
//   }
//   return p;
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

const RegistryIndex int = C.LUA_REGISTRYINDEX

const (
	RegistryIndexMainThread int64 = C.LUA_RIDX_MAINTHREAD
	RegistryIndexGlobals    int64 = C.LUA_RIDX_GLOBALS
)

const (
	LoadedTable  = C.LUA_LOADED_TABLE
	PreloadTable = C.LUA_PRELOAD_TABLE
)

type Type C.int

const (
	TypeNone          Type = C.LUA_TNONE
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

func (l *State) Close() error {
	if l.ptr != nil {
		C.lua_close(l.ptr)
		*l = State{}
	}
	return nil
}

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

func (l *State) Top() int {
	return l.top
}

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

func (l *State) Pop(n int) {
	l.SetTop(-n - 1)
}

func (l *State) PushValue(idx int) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushvalue(l.ptr, C.int(idx))
	l.top++
}

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

func (l *State) Remove(idx int) {
	l.Rotate(idx, -1)
	l.Pop(1)
}

func (l *State) Copy(fromIdx, toIdx int) {
	l.init()
	if !l.isAcceptableIndex(fromIdx) || !l.isAcceptableIndex(toIdx) {
		panic("unacceptable index")
	}
	C.lua_copy(l.ptr, C.int(fromIdx), C.int(toIdx))
}

func (l *State) Replace(idx int) {
	l.Copy(-1, idx)
	l.Pop(1)
}

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

func (l *State) IsNumber(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_isnumber(l.ptr, C.int(idx)) != 0
}

func (l *State) IsString(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_isstring(l.ptr, C.int(idx)) != 0
}

func (l *State) IsNativeFunction(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_iscfunction(l.ptr, C.int(idx)) != 0
}

func (l *State) IsInteger(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_isinteger(l.ptr, C.int(idx)) != 0
}

func (l *State) IsUserdata(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_isuserdata(l.ptr, C.int(idx)) != 0
}

func (l *State) Type(idx int) Type {
	if l.ptr == nil {
		return TypeNone
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return Type(C.lua_type(l.ptr, C.int(idx)))
}

func (l *State) IsFunction(idx int) bool { return l.Type(idx) == TypeFunction }
func (l *State) IsTable(idx int) bool    { return l.Type(idx) == TypeTable }
func (l *State) IsNil(idx int) bool      { return l.Type(idx) == TypeNil }
func (l *State) IsBoolean(idx int) bool  { return l.Type(idx) == TypeBoolean }
func (l *State) IsThread(idx int) bool   { return l.Type(idx) == TypeThread }
func (l *State) IsNone(idx int) bool     { return l.Type(idx) == TypeNone }

func (l *State) IsNoneOrNil(idx int) bool {
	tp := l.Type(idx)
	return tp == TypeNone || tp == TypeNil
}

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

func (l *State) ToBoolean(idx int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return C.lua_toboolean(l.ptr, C.int(idx)) != 0
}

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

func (l *State) RawLen(idx int) uint64 {
	if l.ptr == nil {
		return 0
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return uint64(C.lua_rawlen(l.ptr, C.int(idx)))
}

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

func (l *State) ToPointer(idx int) uintptr {
	if l.ptr == nil {
		return 0
	}
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	return uintptr(C.lua_topointer(l.ptr, C.int(idx)))
}

func (l *State) RawEqual(idx1, idx2 int) bool {
	if l.ptr == nil {
		return false
	}
	if !l.isAcceptableIndex(idx1) || !l.isAcceptableIndex(idx2) {
		panic("unacceptable index")
	}
	return C.lua_rawequal(l.ptr, C.int(idx1), C.int(idx2)) != 0
}

func (l *State) PushNil() {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushnil(l.ptr)
	l.top++
}

func (l *State) PushNumber(n float64) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushnumber(l.ptr, C.lua_Number(n))
	l.top++
}

func (l *State) PushInteger(n int64) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushinteger(l.ptr, C.lua_Integer(n))
	l.top++
}

func (l *State) PushString(s string) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.zombiezen_lua_pushstring(l.ptr, s)
	l.top++
}

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

func (l *State) PushLightUserdata(p uintptr) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.pushlightuserdata(l.ptr, C.uint64_t(p))
	l.top++
}

func (l *State) PushGoValue(v any) {
	if v == nil {
		l.PushNil()
	} else {
		l.init()
		l.pushHandle(cgo.NewHandle(v))
	}
}

type Function = func(*State) (int, error)

func pcall(f Function, l *State) (nResults int, err error) {
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

func (l *State) PushClosure(n int, f Function) {
	if f == nil {
		panic("nil Function")
	}
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

func (l *State) Global(name string, msgHandler int) (Type, error) {
	l.init()
	msgHandler = l.checkMessageHandler(msgHandler)
	l.RawIndex(RegistryIndex, RegistryIndexGlobals)
	tp, err := l.Field(-1, name, msgHandler)
	l.Remove(-2) // remove the globals table
	return tp, err
}

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

func (l *State) RawGet(idx int) Type {
	l.checkElems(1)
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	tp := Type(C.lua_rawget(l.ptr, C.int(idx)))
	return tp
}

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

func (l *State) RawField(idx int, k string) Type {
	idx = l.AbsIndex(idx)
	l.PushString(k)
	return l.RawGet(idx)
}

func (l *State) CreateTable(nArr, nRec int) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_createtable(l.ptr, C.int(nArr), C.int(nRec))
	l.top++
}

func (l *State) NewUserdataUV(nUValue int) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_newuserdatauv(l.ptr, 0, C.int(nUValue))
	l.top++
}

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

func (l *State) RawSet(idx int) {
	l.checkElems(2)
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	C.lua_rawset(l.ptr, C.int(idx))
	l.top -= 2
}

func (l *State) RawSetIndex(idx int, n int64) {
	l.checkElems(1)
	if !l.isAcceptableIndex(idx) {
		panic("unacceptable index")
	}
	C.lua_rawseti(l.ptr, C.int(idx), C.lua_Integer(n))
	l.top--
}

func (l *State) RawSetField(idx int, k string) {
	idx = l.AbsIndex(idx)
	l.PushString(k)
	l.Rotate(-2, 1)
	l.RawSet(idx)
}

func (l *State) SetMetatable(objIndex int) {
	l.checkElems(1)
	if !l.isAcceptableIndex(objIndex) {
		panic("unacceptable index")
	}
	C.lua_setmetatable(l.ptr, C.int(objIndex))
	l.top--
}

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
		return l.newError(ret)
	}
	if newTop >= 0 {
		l.top = newTop
	} else {
		l.top = int(C.lua_gettop(l.ptr))
		l.cap = max(l.cap, l.top)
	}
	return nil
}

const MultipleReturns int = C.LUA_MULTRET

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

	ret := C.lua_load(l.ptr, C.lua_Reader(C.zombiezen_lua_reader), unsafe.Pointer(&handle), chunkNameC, modeC)
	l.top++
	if ret != C.LUA_OK {
		return fmt.Errorf("lua: load %s: %w", formatChunkName(chunkName), l.newError(ret))
	}
	return nil
}

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

func (l *State) Dump(w io.Writer, strip bool) (int64, error) {
	l.checkElems(1)
	state := &writerState{w: cgo.NewHandle(w)}
	defer state.w.Delete()
	stripInt := C.int(0)
	if strip {
		stripInt = 1
	}
	ret := C.lua_dump(l.ptr, C.lua_Writer(C.zombiezen_lua_writercb), unsafe.Pointer(state), stripInt)
	var err error
	switch {
	case state.err != 0:
		err = fmt.Errorf("lua: dump function: %w", state.err.Value().(error))
		state.err.Delete()
	case ret != 0:
		err = fmt.Errorf("lua: dump function: not a function")
	}
	return state.n, err
}

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

func (l *State) Stack(level int) *ActivationRecord {
	l.init()
	ar := new(C.lua_Debug)
	if C.lua_getstack(l.ptr, C.int(level), ar) == 0 {
		return nil
	}
	return &ActivationRecord{
		state: l,
		lptr:  l.ptr,
		ar:    ar,
	}
}

func (l *State) Info(what string) *Debug {
	l.checkElems(1)

	what = strings.TrimPrefix(what, ">")
	cwhat := make([]C.char, 0, len(">\x00")+len(what))
	cwhat = append(cwhat, '>')
	for _, c := range []byte(what) {
		cwhat = append(cwhat, C.char(c))
	}
	cwhat = append(cwhat, 0)

	var tmp C.lua_Debug
	return l.getinfo(&cwhat[0], &tmp)
}

func (l *State) getinfo(what *C.char, ar *C.lua_Debug) *Debug {
	if *what == '>' {
		l.top--
	}

	C.lua_getinfo(l.ptr, what, ar)

	db := &Debug{
		CurrentLine: -1,
	}
	pushFunction := false
	pushLines := false
	for ; *what != 0; what = (*C.char)(unsafe.Add(unsafe.Pointer(what), 1)) {
		switch *what {
		case 'f':
			pushFunction = true
		case 'l':
			db.CurrentLine = int(ar.currentline)
		case 'n':
			if ar.name != nil {
				db.Name = C.GoString(ar.name)
			}
			if ar.namewhat != nil {
				db.NameWhat = C.GoString(ar.namewhat)
			}
		case 'S':
			if ar.what != nil {
				db.What = C.GoString(ar.what)
			}
			if ar.source != nil {
				db.Source = C.GoStringN(ar.source, C.int(ar.srclen))
			}
			db.LineDefined = int(ar.linedefined)
			db.LastLineDefined = int(ar.lastlinedefined)
			db.ShortSource = C.GoString(&ar.short_src[0])
		case 't':
			db.IsTailCall = ar.istailcall != 0
		case 'u':
			db.NumUpvalues = uint8(ar.nups)
			db.NumParams = uint8(ar.nparams)
			db.IsVararg = ar.isvararg != 0
		case 'L':
			pushLines = true
		}
	}
	if pushFunction {
		l.top++
	}
	if pushLines {
		l.top++
	}
	return db
}

type Debug struct {
	Name            string
	NameWhat        string
	What            string
	Source          string
	ShortSource     string
	CurrentLine     int
	LineDefined     int
	LastLineDefined int
	NumUpvalues     uint8
	NumParams       uint8
	IsVararg        bool
	IsTailCall      bool
}

type ActivationRecord struct {
	state *State
	lptr  *C.lua_State
	ar    *C.lua_Debug
}

func (ar *ActivationRecord) isValid() bool {
	return ar != nil && ar.state != nil && ar.state.ptr == ar.lptr
}

func (ar *ActivationRecord) Info(what string) *Debug {
	if strings.HasPrefix(what, ">") {
		panic("what must not start with '>'")
	}
	if !ar.isValid() {
		return nil
	}
	cwhat := C.CString(what)
	defer C.free(unsafe.Pointer(cwhat))
	return ar.state.getinfo(cwhat, ar.ar)
}

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

func PushOpenBase(l *State) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_base), 0)
	l.top++
}

func PushOpenCoroutine(l *State) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_coroutine), 0)
	l.top++
}

func PushOpenTable(l *State) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_table), 0)
	l.top++
}

func PushOpenString(l *State) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_string), 0)
	l.top++
}

func PushOpenUTF8(l *State) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_utf8), 0)
	l.top++
}

func PushOpenMath(l *State) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_math), 0)
	l.top++
}

func PushOpenDebug(l *State) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_debug), 0)
	l.top++
}

func PushOpenPackage(l *State) {
	l.init()
	if l.top >= l.cap {
		panic("stack overflow")
	}
	C.lua_pushcclosure(l.ptr, C.lua_CFunction(C.luaopen_package), 0)
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

// NewMetatable is the auxlib NewMetatable function.
func NewMetatable(l *State, tname string) bool {
	if Metatable(l, tname) != TypeNil {
		// Name already in use.
		return false
	}
	l.Pop(1)
	l.CreateTable(0, 2)
	l.PushString(tname)
	l.RawSetField(-2, "__name") // metatable.__name = tname
	l.PushValue(-1)
	l.RawSetField(RegistryIndex, tname)
	return true
}

// Metatable is the auxlib Metatable function.
func Metatable(l *State, tname string) Type {
	return l.RawField(RegistryIndex, tname)
}

func isPseudo(i int) bool {
	return i <= RegistryIndex
}

const goClosureUpvalueIndex = C.LUA_REGISTRYINDEX - 1

func UpvalueIndex(i int) int {
	if i < 1 || i > 255 {
		panic("invalid upvalue index")
	}
	return C.LUA_REGISTRYINDEX - (i + 1)
}

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
