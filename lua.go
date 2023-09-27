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

/*
Package lua provides low-level bindings for [Lua 5.4].

# Relationship to C API

This package attempts to be a mostly one-to-one mapping with the [Lua C API].
The methods on [State] and [ActivationRecord] are the primitive functions
(i.e. the library functions that start with "lua_").
Functions in this package mostly correspond to the [auxiliary library]
(i.e. the library functions that start with "luaL_"),
but are pure Go reimplementations of these functions,
usually with Go-specific niceties.

[Lua 5.4]: https://www.lua.org/versions.html#5.4
[Lua C API]: https://www.lua.org/manual/5.4/manual.html#4
[auxiliary library]: https://www.lua.org/manual/5.4/manual.html#5
*/
package lua

import (
	"zombiezen.com/go/lua/internal/lua54"
)

// RegistryIndex is a pseudo-index to the [registry],
// a predefined table that can be used by any Go or C code
// to store whatever Lua values it needs to store.
//
// [registry]: https://www.lua.org/manual/5.4/manual.html#4.3
const RegistryIndex int = lua54.RegistryIndex

// MultipleReturns is the option for multiple returns in [State.Call].
const MultipleReturns int = lua54.MultipleReturns

// UpvalueIndex returns the pseudo-index that represents the i-th upvalue
// of the running function.
// If i is outside the range [1, 255], UpvalueIndex panics.
func UpvalueIndex(i int) int {
	return lua54.UpvalueIndex(i)
}

// Predefined keys in the registry.
const (
	// RegistryIndexMainThread is the index at which the registry has the main thread of the state.
	RegistryIndexMainThread int64 = lua54.RegistryIndexMainThread
	// RegistryIndexGlobals is the index at which the registry has the global environment.
	RegistryIndexGlobals int64 = lua54.RegistryIndexGlobals

	// LoadedTable is the key in the registry for the table of loaded modules.
	LoadedTable = lua54.LoadedTable
	// PreloadTable is the key in the registry for the table of preloaded loaders.
	PreloadTable = lua54.PreloadTable
)

// Type is an enumeration of Lua data types.
type Type = lua54.Type

// TypeNone is the value returned from [State.Type]
// for a non-valid but acceptable index.
const TypeNone Type = lua54.TypeNone

// Value types.
const (
	TypeNil           Type = lua54.TypeNil
	TypeBoolean       Type = lua54.TypeBoolean
	TypeLightUserdata Type = lua54.TypeLightUserdata
	TypeNumber        Type = lua54.TypeNumber
	TypeString        Type = lua54.TypeString
	TypeTable         Type = lua54.TypeTable
	TypeFunction      Type = lua54.TypeFunction
	TypeUserdata      Type = lua54.TypeUserdata
	TypeThread        Type = lua54.TypeThread
)

// State represents a Lua execution thread.
// The zero value is a state with a single main thread,
// an empty stack, and an empty environment.
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
type State = lua54.State

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
type Function = lua54.Function

// An ActivationRecord is a reference to a function invocation's activation record.
type ActivationRecord = lua54.ActivationRecord

// Debug holds information about a function or an activation record.
type Debug = lua54.Debug

// Standard library names.
const (
	GName = lua54.GName

	CoroutineLibraryName = lua54.CoroutineLibraryName
	TableLibraryName     = lua54.TableLibraryName
	IOLibraryName        = lua54.IOLibraryName
	OSLibraryName        = lua54.OSLibraryName
	StringLibraryName    = lua54.StringLibraryName
	UTF8LibraryName      = lua54.UTF8LibraryName
	MathLibraryName      = lua54.MathLibraryName
	DebugLibraryName     = lua54.DebugLibraryName
	PackageLibraryName   = lua54.PackageLibraryName
)
