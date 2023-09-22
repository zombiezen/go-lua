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
	state := &State{
		ptr: l,
		top: int(C.lua_gettop(l)),
	}
	state.cap = state.top + C.LUA_MINSTACK

	ptr := (*cgo.Handle)(C.lua_touserdata(l, upvalueIndex(1)))
	f := ptr.Value().(Function)
	results, err := f.pcall(state)
	if err != nil {
		C.zombiezen_lua_pushstring(l, err.Error())
		return -1
	}
	if results < 0 {
		return 0
	}
	return C.int(results)
}

//export zombiezen_lua_gchandle
func zombiezen_lua_gchandle(l *C.lua_State) C.int {
	ptr := (*cgo.Handle)(C.lua_touserdata(l, 1))
	ptr.Delete()
	return 0
}

// upvalueIndex returns the pseudo-index that represents the i-th upvalue
// of the running function.
// i must be in the range [1,256].
func upvalueIndex(i C.int) C.int {
	return C.LUA_REGISTRYINDEX - i
}
