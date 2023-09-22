package lua

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	state := new(State)
	defer func() {
		if err := state.Close(); err != nil {
			t.Error("Close:", err)
		}
	}()

	const source = "return 2 + 2"
	if err := state.Load(strings.NewReader(source), source, "t"); err != nil {
		t.Fatal(err)
	}
	if err := state.Call(0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if !state.IsNumber(-1) {
		t.Fatalf("top of stack is %v; want number", state.Type(-1))
	}
	const want = int64(4)
	if got, ok := state.ToInteger(-1); got != want || !ok {
		t.Errorf("state.ToInteger(-1) = %d, %t; want %d, true", got, ok, want)
	}
}

func TestLoadString(t *testing.T) {
	state := new(State)
	defer func() {
		if err := state.Close(); err != nil {
			t.Error("Close:", err)
		}
	}()

	const source = "return 2 + 2"
	if err := state.LoadString(source, source, "t"); err != nil {
		t.Fatal(err)
	}
	if err := state.Call(0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if !state.IsNumber(-1) {
		t.Fatalf("top of stack is %v; want number", state.Type(-1))
	}
	const want = int64(4)
	if got, ok := state.ToInteger(-1); got != want || !ok {
		t.Errorf("state.ToInteger(-1) = %d, %t; want %d, true", got, ok, want)
	}
}

func TestPushFunction(t *testing.T) {
	state := new(State)
	defer func() {
		if err := state.Close(); err != nil {
			t.Error("Close:", err)
		}
	}()

	const want = 42
	state.PushFunction(func(l *State) (int, error) {
		l.PushInteger(want)
		return 1, nil
	})
	if err := state.Call(0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if got, ok := state.ToInteger(-1); got != want || !ok {
		value, err := ToString(state, -1)
		if err != nil {
			value = "<unknown value>"
		}
		t.Errorf("function returned %s; want %d", value, err)
	}
}

func TestBasicLibrary(t *testing.T) {
	state := new(State)
	defer func() {
		if err := state.Close(); err != nil {
			t.Error("Close:", err)
		}
	}()

	out := new(bytes.Buffer)
	state.PushOpenBase(out)
	if err := Require(state, GName, true); err != nil {
		t.Error(err)
	}
	if _, err := state.Global("print", 0); err != nil {
		t.Fatal(err)
	}
	state.PushString("Hello, World!")
	if err := state.Call(1, 0, 0); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "Hello, World!\n"; got != want {
		t.Errorf("output = %q; want %q", got, want)
	}
}

func BenchmarkExec(b *testing.B) {
	state := new(State)
	defer func() {
		if err := state.Close(); err != nil {
			b.Error("Close:", err)
		}
	}()

	const source = "return 2 + 2"
	for i := 0; i < b.N; i++ {
		if err := state.LoadString(source, source, "t"); err != nil {
			b.Fatal(err)
		}
		if err := state.Call(0, 1, 0); err != nil {
			b.Fatal(err)
		}
		state.Pop(1)
	}
}
