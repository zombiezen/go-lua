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
