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

package lua_test

import (
	"log"
	"os"

	"zombiezen.com/go/lua"
)

func Example() {
	// Create an execution environment
	// and make the standard libraries available.
	state := new(lua.State)
	defer state.Close()
	if err := lua.OpenLibraries(state, os.Stdout); err != nil {
		log.Fatal(err)
	}

	// Load Lua code as a chunk/function.
	// Calling this function then executes it.
	const luaSource = `print("Hello, World!")`
	if err := state.LoadString(luaSource, luaSource, "t"); err != nil {
		log.Fatal(err)
	}
	if err := state.Call(0, 0, 0); err != nil {
		log.Fatal(err)
	}
	// Output:
	// Hello, World!
}
