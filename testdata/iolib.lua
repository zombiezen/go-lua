-- Copyright 2023 Ross Light
--
-- Permission is hereby granted, free of charge, to any person obtaining a copy of
-- this software and associated documentation files (the “Software”), to deal in
-- the Software without restriction, including without limitation the rights to
-- use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
-- the Software, and to permit persons to whom the Software is furnished to do so,
-- subject to the following conditions:
--
-- The above copyright notice and this permission notice shall be included in all
-- copies or substantial portions of the Software.
--
-- THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
-- IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
-- FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
-- COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
-- IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
-- CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
--
-- SPDX-License-Identifier: MIT

local f = assert(io.open("foo.txt", "w"))
local wresult = assert(f:write("Hello, ", 42, "!\n"))
assert(wresult == f, "write result is "..tostring(wresult))
wresult = assert(f:write("second line\n"))
assert(wresult == f, "write result is "..tostring(wresult))
assert(f:close())

f = assert(io.open("foo.txt"))
local line1 = assert(f:read())
assert(line1 == "Hello, 42!")
local rest = assert(f:read("a"))
assert(rest == "second line\n")
assert(f:close())
