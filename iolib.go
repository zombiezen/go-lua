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
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ioInput  = "_zombiezen_IO_input"
	ioOutput = "_zombiezen_IO_output"
)

// IOLibrary is a pure Go implementation of the standard Lua "io" library.
// The zero value of IOLibrary stubs out all functionality.
type IOLibrary struct {
	// Stdin is the reader for io.stdin.
	// If nil, stdin will act like an empty file.
	Stdin io.Reader
	// Stdout is the writer for io.stdout.
	// If nil, io.stdout will discard any data written to it.
	Stdout io.Writer
	// Stderr is the writer for io.stderr.
	// If nil, io.stderr will discard any data written to it.
	Stderr io.Writer

	// Open opens a file with the given name and [mode].
	// The returned file should implement io.Reader and/or io.Writer,
	// but may optionally implement io.Seeker.
	//
	// [mode]: https://www.lua.org/manual/5.4/manual.html#pdf-io.open
	Open func(name, mode string) (io.Closer, error)

	// CreateTemp returns a handle for a temporary file opened in update mode.
	// The returned file should clean up the file on Close.
	CreateTemp func() (ReadWriteSeekCloser, error)
}

// NewIOLibrary returns an OSLibrary that uses the native operating system.
func NewIOLibrary() *IOLibrary {
	return &IOLibrary{
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Open:       ioOpen,
		CreateTemp: ioCreateTemp,
	}
}

func ioOpen(name, mode string) (io.Closer, error) {
	var flag int
	mode = strings.TrimSuffix(mode, "b")
	switch mode {
	case "r":
		flag = os.O_RDONLY
	case "w":
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "a":
		flag = os.O_WRONLY | os.O_APPEND | os.O_CREATE
	case "r+":
		flag = os.O_RDWR | os.O_CREATE
	case "w+":
		flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	case "a+":
		flag = os.O_RDWR | os.O_APPEND | os.O_CREATE
	default:
		return nil, &os.PathError{
			Op:   "open",
			Path: name,
			Err:  fmt.Errorf("invalid mode %q", mode),
		}
	}
	return os.OpenFile(name, flag, 0o666)
}

func ioCreateTemp() (ReadWriteSeekCloser, error) {
	f, err := os.CreateTemp("", "lua_")
	if err != nil {
		return nil, err
	}
	fname := f.Name()
	if runtime.GOOS != "windows" {
		// Non-Windows operating systems usually support unlinking the file while it's open.
		// If it works, then that's all we have to do.
		if err := os.Remove(fname); err == nil {
			return f, nil
		}
	}
	fullPath, err := filepath.Abs(fname)
	if err != nil {
		f.Close()
		os.Remove(fname)
		return nil, err
	}
	return &removeOnCloseFile{f, fullPath}, nil
}

// OpenLibrary loads the standard io library.
// This method is intended to be used as an argument to [Require].
func (lib *IOLibrary) OpenLibrary(l *State) (int, error) {
	err := NewLib(l, map[string]Function{
		"close":   lib.close,
		"input":   lib.input,
		"open":    lib.open,
		"output":  lib.output,
		"read":    lib.read,
		"stderr":  nil,
		"stdin":   nil,
		"stdout":  nil,
		"tmpfile": lib.tmpfile,
		"type":    lib.type_,
		"write":   lib.write,
	})
	if err != nil {
		return 0, err
	}
	if err := lib.createMetatable(l); err != nil {
		return 0, err
	}

	stdinStream := &stream{c: noClose{}}
	if lib.Stdin != nil {
		stdinStream.r = bufio.NewReader(lib.Stdin)
	}
	pushStream(l, stdinStream)
	l.PushValue(-1)
	if err := l.SetField(RegistryIndex, ioInput, 0); err != nil {
		return 0, err
	}
	l.RawSetField(-2, "stdin")

	pushStream(l, &stream{w: lib.Stdout, c: noClose{}})
	l.PushValue(-1)
	if err := l.SetField(RegistryIndex, ioOutput, 0); err != nil {
		return 0, err
	}
	l.RawSetField(-2, "stdout")

	pushStream(l, &stream{w: lib.Stderr, c: noClose{}})
	l.RawSetField(-2, "stderr")

	return 1, nil
}

const streamMetatableName = "*zombiezen.com/go/lua.stream"

func (lib *IOLibrary) createMetatable(l *State) error {
	NewMetatable(l, streamMetatableName)
	err := SetFuncs(l, 0, map[string]Function{
		"__index":    nil,
		"__gc":       lib.fgc,
		"__close":    nil,
		"__tostring": lib.ftostring,
	})
	if err != nil {
		return err
	}

	// Use same __gc function for __close.
	l.RawField(-1, "__gc")
	l.RawSetField(-2, "__close")

	err = NewLib(l, map[string]Function{
		"close": lib.fclose,
		"read":  lib.fread,
		"write": lib.fwrite,
	})
	if err != nil {
		return err
	}
	l.RawSetField(-2, "__index") // metatable.__index = method table

	l.Pop(1)
	return nil
}

func (lib *IOLibrary) type_(l *State) (int, error) {
	const argIdx = 1
	if l.IsNone(argIdx) {
		return 0, NewArgError(l, argIdx, "value expected")
	}
	if !TestUserdata(l, argIdx, streamMetatableName) {
		pushFail(l)
		return 1, nil
	}
	l.UserValue(argIdx, 1)
	s, _ := l.ToGoValue(-1).(*stream)
	if s == nil {
		pushFail(l)
		return 1, nil
	}
	if s.isClosed() {
		l.PushString("closed file")
	} else {
		l.PushString("file")
	}
	return 1, nil
}

func (lib *IOLibrary) ftostring(l *State) (int, error) {
	s, err := toStream(l)
	if err != nil {
		return 0, err
	}
	switch {
	case s.isClosed():
		l.PushString("file (closed)")
	case s.r != nil:
		l.PushString(fmt.Sprintf("file (%p)", s.r))
	case s.w != nil:
		l.PushString(fmt.Sprintf("file (%p)", s.w))
	default:
		l.PushString("file")
	}
	return 1, nil
}

func (lib *IOLibrary) fclose(l *State) (int, error) {
	s, err := toStream(l)
	if err != nil {
		return 0, err
	}
	err = s.Close()
	return pushFileResult(l, err), nil
}

func (lib *IOLibrary) close(l *State) (int, error) {
	if l.IsNone(1) {
		// Use default output.
		if _, err := l.Field(RegistryIndex, ioOutput, 0); err != nil {
			return 0, err
		}
	}
	return lib.fclose(l)
}

func (lib *IOLibrary) fgc(l *State) (int, error) {
	s, err := toStream(l)
	if err != nil {
		return 0, err
	}
	s.Close()
	return 0, nil
}

func (lib *IOLibrary) open(l *State) (int, error) {
	filename, err := CheckString(l, 1)
	if err != nil {
		return 0, err
	}
	mode := "r"
	if !l.IsNoneOrNil(2) {
		mode, err = CheckString(l, 2)
		if err != nil {
			return 0, err
		}
	}
	s, err := lib.doOpen(filename, mode)
	if err != nil {
		return pushFileResult(l, err), nil
	}
	pushStream(l, s)
	return 1, nil
}

func (lib *IOLibrary) doOpen(filename, mode string) (*stream, error) {
	if lib.Open == nil {
		return nil, errors.ErrUnsupported
	}
	f, err := lib.Open(filename, mode)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errors.New("IOLibrary.Open returned nil")
	}
	s := &stream{c: f}
	r, _ := f.(io.Reader)
	if r != nil {
		s.r = bufio.NewReader(r)
	}
	s.w, _ = f.(io.Writer)
	s.seek, _ = f.(io.Seeker)
	return s, nil
}

func (lib *IOLibrary) tmpfile(l *State) (int, error) {
	if lib.CreateTemp == nil {
		return pushFileResult(l, errors.ErrUnsupported), nil
	}
	f, err := lib.CreateTemp()
	if err != nil {
		return pushFileResult(l, err), nil
	}
	if f == nil {
		return pushFileResult(l, errors.New("IOLibrary.CreateTemp returned nil")), nil
	}
	pushStream(l, &stream{
		r:    bufio.NewReader(f),
		w:    f,
		seek: f,
		c:    f,
	})
	return 1, nil
}

func (lib *IOLibrary) input(l *State) (int, error) {
	return lib.filefunc(l, ioInput, "r")
}

func (lib *IOLibrary) output(l *State) (int, error) {
	return lib.filefunc(l, ioOutput, "w")
}

// filefunc implements io.input and io.output.
func (lib *IOLibrary) filefunc(l *State, f, mode string) (int, error) {
	if !l.IsNoneOrNil(1) {
		if filename, ok := l.ToString(1); ok {
			s, err := lib.doOpen(filename, mode)
			if err != nil {
				return 0, fmt.Errorf("%s%w", Where(l, 1), err)
			}
			pushStream(l, s)
		} else {
			if _, err := toStream(l); err != nil {
				return 0, err
			}
			l.PushValue(1)
		}
		if err := l.SetField(RegistryIndex, f, 0); err != nil {
			return 0, err
		}
	}
	if _, err := l.Field(RegistryIndex, f, 0); err != nil {
		return 0, err
	}
	return 1, nil
}

func (lib *IOLibrary) read(l *State) (int, error) {
	s, err := registryStream(l, ioInput)
	if err != nil {
		return 0, err
	}
	return lib.doRead(l, s, 1)
}

func (lib *IOLibrary) fread(l *State) (int, error) {
	s, err := toStream(l)
	if err != nil {
		return 0, err
	}
	return lib.doRead(l, s, 2)
}

// doWrite handles the io.write function or file:write method.
// first is the 1-based first format to read.
// It is assumed that the stream object is at either top or bottom of the stack.
func (lib *IOLibrary) doRead(l *State, s *stream, first int) (int, error) {
	nArgs := l.Top() - 1
	if nArgs <= 0 {
		line, err := s.readLine(true)
		if err == io.EOF {
			pushFail(l)
			return 1, nil
		}
		if err != nil {
			return pushFileResult(l, err), nil
		}
		l.PushString(line)
		return 1, nil
	}

	if !l.CheckStack(nArgs + 20) {
		return 0, fmt.Errorf("%sstack overflow (too many arguments)", Where(l, 1))
	}
	var n int
	for n = first; nArgs > 0; n, nArgs = n+1, nArgs-1 {
		if l.Type(n) == TypeNumber {
			size, err := CheckInteger(l, n)
			if err != nil {
				return 0, err
			}
			if size < 0 || size > math.MaxInt {
				return 0, NewArgError(l, n, "out of range")
			}
			buf, err := s.read(int(size))
			if err == io.EOF {
				pushFail(l)
				break
			}
			if err != nil {
				return pushFileResult(l, err), nil
			}
			// TODO(someday): Push bytes directly.
			l.PushString(string(buf))
			continue
		}

		format, err := CheckString(l, n)
		if err != nil {
			return 0, err
		}
		format = strings.TrimPrefix(format, "*")
		switch format {
		case "l", "L":
			line, err := s.readLine(format == "l")
			if err == io.EOF {
				pushFail(l)
				break
			}
			if err != nil {
				return pushFileResult(l, err), nil
			}
			l.PushString(line)
		case "a":
			l.PushString(s.readAll())
		default:
			return 0, NewArgError(l, n, "invalid format")
		}
	}
	return n - first, nil
}

func (lib *IOLibrary) write(l *State) (int, error) {
	s, err := registryStream(l, ioOutput)
	if err != nil {
		return 0, err
	}
	return lib.doWrite(l, s, 1)
}

func (lib *IOLibrary) fwrite(l *State) (int, error) {
	s, err := toStream(l)
	if err != nil {
		return 0, err
	}
	l.PushValue(1) // push file at the stack top (to be returned)
	return lib.doWrite(l, s, 2)
}

// doWrite handles the io.write function or file:write method.
// The top of the stack must be the file handle object.
// arg is the 1-based first argument to write.
func (lib *IOLibrary) doWrite(l *State, s *stream, arg int) (int, error) {
	nArgs := l.Top() - arg
	for ; nArgs > 0; arg, nArgs = arg+1, nArgs-1 {
		var werr error
		if l.Type(arg) == TypeNumber {
			if l.IsInteger(arg) {
				n, _ := l.ToInteger(arg)
				_, werr = fmt.Fprintf(s.w, "%d", n)
			} else {
				n, _ := l.ToNumber(arg)
				_, werr = fmt.Fprintf(s.w, "%.14g", n)
			}
		} else {
			var argString string
			argString, err := CheckString(l, arg)
			if err != nil {
				return 0, err
			}
			_, werr = io.WriteString(s.w, argString)
		}
		if werr != nil {
			return pushFileResult(l, werr), nil
		}
	}
	// File handle already on stack top.
	return 1, nil
}

type stream struct {
	r    *bufio.Reader
	w    io.Writer
	seek io.Seeker
	c    io.Closer
}

func pushStream(l *State, s *stream) {
	l.NewUserdataUV(1)
	l.PushGoValue(s)
	l.SetUserValue(-2, 1)
	SetMetatable(l, streamMetatableName)
}

// registryStream gets the stream stored in the registry at the given key
// and pushes it onto the stack.
func registryStream(l *State, findex string) (*stream, error) {
	if _, err := l.Field(RegistryIndex, findex, 0); err != nil {
		return nil, err
	}
	if err := CheckUserdata(l, -1, streamMetatableName); err != nil {
		return nil, err
	}
	l.UserValue(-1, 1)
	s, _ := l.ToGoValue(-1).(*stream)
	if s == nil {
		return nil, fmt.Errorf("could not extract stream from registry %q", findex)
	}
	return s, nil
}

func toStream(l *State) (*stream, error) {
	const idx = 1
	if err := CheckUserdata(l, idx, streamMetatableName); err != nil {
		return nil, err
	}
	l.UserValue(idx, 1)
	s, _ := l.ToGoValue(-1).(*stream)
	l.Pop(1)
	if s == nil {
		return nil, NewArgError(l, idx, "could not extract stream")
	}
	return s, nil
}

func (s *stream) read(n int) ([]byte, error) {
	if n == 0 {
		if _, err := s.r.Peek(1); err != nil {
			return nil, io.EOF
		}
		return nil, nil
	}
	buf := make([]byte, n)
	n, err := s.r.Read(buf)
	if n == 0 {
		if err == nil {
			return nil, io.ErrNoProgress
		}
		return nil, err
	}
	return buf[:n], nil
}

func (s *stream) readAll() string {
	// TODO(someday): Add limits.
	sb := new(strings.Builder)
	_, _ = io.Copy(sb, s.r)
	return sb.String()
}

func (s *stream) readLine(chop bool) (string, error) {
	sb := new(strings.Builder)
	for {
		b, err := s.r.ReadByte()
		if err != nil {
			if sb.Len() == 0 {
				return "", err
			}
			return sb.String(), nil
		}
		if b == '\n' {
			if !chop {
				sb.WriteByte(b)
			}
			return sb.String(), nil
		}
		sb.WriteByte(b)
	}
}

func (s *stream) isClosed() bool {
	return s.c == nil
}

func (s *stream) Close() error {
	if s.isClosed() {
		return nil
	}
	err := s.c.Close()
	*s = stream{}
	return err
}

// ReadWriteSeekCloser is an interface
// that groups the basic Read, Write, Seek, and Close methods.
type ReadWriteSeekCloser interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
}

type noClose struct{}

func (noClose) Close() error {
	return errors.New("cannot close standard file")
}

type removeOnCloseFile struct {
	f    *os.File
	path string
}

func (f *removeOnCloseFile) Read(p []byte) (int, error) {
	return f.f.Read(p)
}

func (f *removeOnCloseFile) Write(p []byte) (int, error) {
	return f.f.Write(p)
}

func (f *removeOnCloseFile) Seek(offset int64, whence int) (int64, error) {
	return f.f.Seek(offset, whence)
}

func (f *removeOnCloseFile) Close() error {
	err1 := f.f.Close()
	err2 := os.Remove(f.path)
	if err1 != nil {
		return err1
	}
	return err2
}
