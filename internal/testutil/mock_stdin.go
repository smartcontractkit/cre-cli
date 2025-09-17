package testutil

import "io"

// MockStdinReader is a simple io.Reader that returns one line (with a newline) at a time.
type MockStdinReader struct {
	lines   []string // each line without a trailing newline
	curLine int      // index of current line
	curPos  int      // current position within the current line (including the appended newline)
}

func NewMockStdinReader(lines []string) *MockStdinReader {
	return &MockStdinReader{lines: lines}
}

func SingleMockStdinReader(line string) *MockStdinReader {
	return &MockStdinReader{lines: []string{line}}
}

func EmptyMockStdinReader() *MockStdinReader {
	return &MockStdinReader{lines: []string{""}}
}

// Read implements the io.Reader interface.
func (r *MockStdinReader) Read(p []byte) (n int, err error) {
	// If we've consumed all lines, return EOF.
	if r.curLine >= len(r.lines) {
		return 0, io.EOF
	}

	// Append a newline to the current line.
	current := r.lines[r.curLine] + "\n"
	remaining := []byte(current)[r.curPos:]

	// Copy as much as we can into p.
	n = copy(p, remaining)
	r.curPos += n

	// If we have fully read the current line, move to the next.
	if r.curPos >= len(current) {
		r.curLine++
		r.curPos = 0
	}

	return n, nil
}
