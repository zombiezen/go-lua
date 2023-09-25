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
	"runtime/cgo"
	"unsafe"
)

// #include <stdlib.h>
// #include <stddef.h>
// #include "lua.h"
//
// void zombiezen_lua_pushstring(lua_State *L, _GoString_ s);
import "C"

//export zombiezen_lua_readercb
func zombiezen_lua_readercb(l *C.lua_State, data unsafe.Pointer, size *C.size_t) *C.char {
	r := (*cgo.Handle)(data).Value().(*reader)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(r.buf)), readerBufferSize)
	n, _ := r.r.Read(buf)
	*size = C.size_t(n)
	return r.buf
}

//export zombiezen_lua_gocb
func zombiezen_lua_gocb(l *C.lua_State) C.int {
	state := stateForCallback(l)
	defer func() {
		// Once the callback has finished, clear the State.
		// This prevents incorrect usage, especially with ActivationRecords.
		*state = State{}
	}()
	handlePtr := state.testHandle(goClosureUpvalueIndex)
	if handlePtr == nil || *handlePtr == 0 {
		C.zombiezen_lua_pushstring(l, "Go closure upvalue corrupted")
		return -1
	}
	f, ok := handlePtr.Value().(Function)
	if !ok {
		C.zombiezen_lua_pushstring(l, "Go closure upvalue corrupted")
		return -1
	}

	results, err := f.pcall(state)
	if err != nil {
		C.zombiezen_lua_pushstring(l, err.Error())
		return -1
	}
	if results < 0 {
		C.zombiezen_lua_pushstring(l, "Go callback returned negative results")
		return -1
	}
	return C.int(results)
}

//export zombiezen_lua_gchandle
func zombiezen_lua_gchandle(l *C.lua_State) C.int {
	state := stateForCallback(l)
	ptr := state.testHandle(1)
	if ptr != nil && *ptr != 0 {
		ptr.Delete()
		*ptr = 0
	}
	return 0
}
