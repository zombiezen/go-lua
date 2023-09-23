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

func TestPushGoValue(t *testing.T) {
	tests := []struct {
		name string
		v    any
		tp   Type
	}{
		{
			name: "Nil",
			v:    nil,
			tp:   TypeNil,
		},
		{
			name: "Number",
			v:    42,
			tp:   TypeUserdata,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := new(State)
			defer func() {
				if err := state.Close(); err != nil {
					t.Error("Close:", err)
				}
			}()

			state.PushGoValue(test.v)
			if got, want := state.Top(), 1; got != want {
				t.Fatalf("state.Top() = %d; want %d", got, want)
			}
			if got := state.Type(-1); got != test.tp {
				t.Errorf("state.Type(-1) = %v; want %v", got, test.tp)
			}
			if got := state.ToGoValue(-1); got != test.v {
				t.Errorf("state.ToGoValue(-1) = %#v; want %#v", got, test.v)
			}
		})
	}
}

func TestInvalidToGoValue(t *testing.T) {
	state := new(State)
	defer func() {
		if err := state.Close(); err != nil {
			t.Error("Close:", err)
		}
	}()

	state.NewUserdataUV(0)
	if got := state.ToGoValue(1); got != nil {
		t.Errorf("state.ToGoValue(1) = %#v; want <nil>", got)
	}
}

func TestFullUserdata(t *testing.T) {
	state := new(State)
	defer func() {
		if err := state.Close(); err != nil {
			t.Error("Close:", err)
		}
	}()

	const want = 42
	state.NewUserdataUV(1)
	state.PushInteger(want)
	if !state.SetUserValue(-2, 1) {
		t.Error("Userdata does not have value 1")
	}
	if got, want := state.UserValue(-1, 1), TypeNumber; got != want {
		t.Errorf("user value 1 type = %v; want %v", got, want)
	}
	if got, ok := state.ToInteger(-1); got != want || !ok {
		value, err := ToString(state, -1)
		if err != nil {
			value = "<unknown value>"
		}
		t.Errorf("user value 1 = %s; want %d", value, want)
	}
	state.Pop(1)

	if got, want := state.UserValue(-1, 2), TypeNone; got != want {
		t.Errorf("user value 2 type = %v; want %v", got, want)
	}
	if got, want := state.Top(), 2; got != want {
		t.Errorf("after state.UserValue(-1, 2), state.Top() = %d; want %d", got, want)
	}
	if !state.IsNil(-1) {
		value, err := ToString(state, -1)
		if err != nil {
			value = "<unknown value>"
		}
		t.Errorf("user value 2 = %s; want nil", value)
	}
}

func TestLightUserdata(t *testing.T) {
	state := new(State)
	defer func() {
		if err := state.Close(); err != nil {
			t.Error("Close:", err)
		}
	}()

	vals := []uintptr{0, 42}
	for _, p := range vals {
		state.PushLightUserdata(p)
	}

	if got, want := state.Top(), len(vals); got != want {
		t.Fatalf("state.Top() = %d; want %d", got, want)
	}
	for i := 1; i <= len(vals); i++ {
		if got, want := state.Type(i), TypeLightUserdata; got != want {
			t.Errorf("state.Type(%d) = %v; want %v", i, got, want)
		}
		if !state.IsUserdata(i) {
			t.Errorf("state.IsUserdata(%d) = false; want true", i)
		}
		if got, want := state.ToPointer(i), vals[i-1]; got != want {
			t.Errorf("state.ToPointer(%d) = %#x; want %#x", i, got, want)
		}
	}
}

func TestPushClosure(t *testing.T) {
	t.Run("NoUpvalues", func(t *testing.T) {
		state := new(State)
		defer func() {
			if err := state.Close(); err != nil {
				t.Error("Close:", err)
			}
		}()

		const want = 42
		state.PushClosure(0, func(l *State) (int, error) {
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
			t.Errorf("function returned %s; want %d", value, want)
		}
	})

	t.Run("Upvalues", func(t *testing.T) {
		state := new(State)
		defer func() {
			if err := state.Close(); err != nil {
				t.Error("Close:", err)
			}
		}()

		const want = 42
		state.PushInteger(want)
		state.PushClosure(1, func(l *State) (int, error) {
			l.PushValue(UpvalueIndex(1))
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
			t.Errorf("function returned %s; want %d", value, want)
		}
	})
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
	state.PushInteger(42)
	if err := state.Call(2, 0, 0); err != nil {
		t.Fatal(err)
	}

	if got, want := out.String(), "Hello, World!\t42\n"; got != want {
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
