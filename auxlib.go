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
	"fmt"
	"strconv"
)

// Metafield pushes onto the stack the field event
// from the metatable of the object at index obj
// and returns the type of the pushed value.
// If the object does not have a metatable,
// or if the metatable does not have this field,
// pushes nothing and returns [TypeNil].
func Metafield(l *State, obj int, event string) Type {
	if !l.Metatable(obj) {
		return TypeNil
	}
	l.PushString(event)
	tt := l.RawGet(-2)
	if tt == TypeNil {
		l.Pop(2) // remove metatable and metafield
	} else {
		l.Remove(-2) // remove only metatable
	}
	return tt
}

// CallMeta calls a metamethod.
//
// If the object at index obj has a metatable and this metatable has a field event,
// this function calls this field passing the object as its only argument.
// In this case this function returns true
// and pushes onto the stack the value returned by the call.
// If an error is raised during the call,
// CallMeta returns an error without pushing any value on the stack.
// If there is no metatable or no metamethod,
// this function returns false without pushing any value on the stack.
func CallMeta(l *State, obj int, event string) (bool, error) {
	obj = l.AbsIndex(obj)
	if Metafield(l, obj, event) == TypeNil {
		// No metafield.
		return false, nil
	}
	l.PushValue(obj)
	if err := l.Call(1, 1, 0); err != nil {
		l.Pop(1)
		return true, fmt.Errorf("lua: call metafield %q: %w", event, unwrapError(err))
	}
	return true, nil
}

// ToString converts any Lua value at the given index
// to a Go string in a reasonable format.
//
// If the value has a metatable with a __tostring field,
// then ToString calls the corresponding metamethod with the value as argument,
// and uses the result of the call as its result.
func ToString(l *State, idx int) (string, error) {
	idx = l.AbsIndex(idx)
	if hasMethod, err := CallMeta(l, idx, "__tostring"); err != nil {
		return "", err
	} else if hasMethod {
		if !l.IsString(-1) {
			l.Pop(1)
			return "", fmt.Errorf("lua: '__tostring' must return a string")
		}
		s, _ := l.ToString(idx)
		l.Pop(1)
		return s, nil
	}

	switch l.Type(idx) {
	case TypeNumber:
		if l.IsInteger(idx) {
			n, _ := l.ToInteger(idx)
			return strconv.FormatInt(n, 10), nil
		}
		n, _ := l.ToNumber(idx)
		return strconv.FormatFloat(n, 'g', -1, 64), nil
	case TypeString:
		s, _ := l.ToString(idx)
		return s, nil
	case TypeBoolean:
		if l.ToBoolean(idx) {
			return "true", nil
		} else {
			return "false", nil
		}
	case TypeNil:
		return "nil", nil
	default:
		var kind string
		if tt := Metafield(l, idx, "__name"); tt == TypeString {
			kind, _ = l.ToString(-1)
			l.Pop(1)
		} else {
			if tt != TypeNil {
				l.Pop(1)
			}
			kind = l.Type(idx).String()
		}
		return fmt.Sprintf("%s: %#x", kind, l.ToPointer(idx)), nil
	}
}

// CheckString checks whether the function argument arg is a string
// and returns this string.
// This function uses [State.ToString] to get its result,
// so all conversions and caveats of that function apply here.
func CheckString(l *State, arg int) (string, error) {
	s, ok := l.ToString(arg)
	if !ok {
		return "", NewTypeError(l, arg, TypeString.String())
	}
	return s, nil
}

// CheckInteger checks whether the function argument arg is an integer
// (or can be converted to an integer)
// and returns this integer.
func CheckInteger(l *State, arg int) (int64, error) {
	d, ok := l.ToInteger(arg)
	if !ok {
		if l.IsNumber(arg) {
			return 0, NewArgError(l, arg, "number has no integer representation")
		}
		return 0, NewTypeError(l, arg, TypeNumber.String())
	}
	return d, nil
}

// NewMetatable gets or creates a table in the registry
// to be used as a metatable for userdata.
// If the table is created, adds the pair __name = tname,
// and returns true.
// Regardless, the function pushes onto the stack
// the final value associated with tname in the registry.
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

// Metatable pushes onto the stack the metatable associated with the name tname
// in the registry (see [NewMetatable]),
// or nil if there is no metatable associated with that name.
// Returns the type of the pushed value.
func Metatable(l *State, tname string) Type {
	return l.RawField(RegistryIndex, tname)
}

// SetMetatable sets the metatable of the object on the top of the stack
// as the metatable associated with name tname in the registry.
// [NewMetatable] can be used to create such a metatable.
func SetMetatable(l *State, tname string) {
	Metatable(l, tname)
	l.SetMetatable(-2)
}

// Subtable ensures that the value t[fname],
// where t is the value at index idx, is a table,
// and pushes that table onto the stack.
// Returns true if it finds a previous table there
// and false if it creates a new table.
func Subtable(l *State, idx int, fname string) (bool, error) {
	tp, err := l.Field(idx, fname, 0)
	if err != nil {
		l.Pop(1) // pop error value
		return false, err
	}
	if tp == TypeTable {
		return true, nil
	}
	l.Pop(1)
	idx = l.AbsIndex(idx)
	l.CreateTable(0, 0)
	l.PushValue(-1) // copy to be left at top
	err = l.SetField(idx, fname, 0)
	if err != nil {
		l.Pop(2) // pop table and error value
		return false, err
	}
	return false, nil
}

// Require loads a module using the given openf function.
// If package.loaded[modName] is not true,
// Require calls the function with the string modName as an argument
// and sets the call result to package.loaded[modName],
// as if that function has been called through require.
// If global is true, also stores the module into the global modName.
// Leaves a copy of the module on the stack.
func Require(l *State, modName string, global bool, openf Function) error {
	if _, err := Subtable(l, RegistryIndex, LoadedTable); err != nil {
		return fmt.Errorf("lua: require %q: %w", modName, err)
	}
	if _, err := l.Field(-1, modName, 0); err != nil {
		return fmt.Errorf("lua: require %q: %w", modName, err)
	}
	if !l.ToBoolean(-1) {
		l.Pop(1) // remove field
		l.PushClosure(0, openf)
		l.PushString(modName)
		if err := l.Call(1, 1, 0); err != nil {
			return fmt.Errorf("lua: require %q: %w", modName, err)
		}
		l.PushValue(-1)
		if err := l.SetField(-3, modName, 0); err != nil {
			return fmt.Errorf("lua: require %q: %w", modName, err)
		}
	}
	l.Remove(-2) // remove LOADED table
	if global {
		l.PushValue(-1) // copy of module
		if err := l.SetGlobal(modName, 0); err != nil {
			return fmt.Errorf("lua: require %q: %w", modName, err)
		}
	}
	return nil
}
