package main

import (
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"golang.org/x/term"
)

// Editor is a minimal line editor with cursor tracking.
// It reads from /dev/tty so it works even when stdout is redirected.
type Editor struct {
	tty      *os.File
	oldState *term.State
	buf      []byte
	pos      int // cursor byte offset into buf
}

// NewEditor opens /dev/tty and switches to raw mode.
func NewEditor() (*Editor, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/tty: %w", err)
	}

	old, err := term.MakeRaw(int(tty.Fd()))
	if err != nil {
		tty.Close()
		return nil, fmt.Errorf("raw mode: %w", err)
	}

	return &Editor{tty: tty, oldState: old}, nil
}

// Close restores terminal state and closes the tty fd.
func (e *Editor) Close() {
	term.Restore(int(e.tty.Fd()), e.oldState)
	e.tty.Close()
}

// Tty returns the tty file for writing prompts/UI.
func (e *Editor) Tty() *os.File {
	return e.tty
}

// ReadLine displays the prompt and reads a line with full cursor tracking.
// Returns the input text, cursor position, and whether input was received.
// Returns io.EOF when the user presses Ctrl-D on empty input.
func (e *Editor) ReadLine(prompt string) (text string, cursor int, err error) {
	e.buf = e.buf[:0]
	e.pos = 0
	e.redraw(prompt)

	var esc [8]byte // buffer for escape sequences

	for {
		var b [1]byte
		_, err := e.tty.Read(b[:])
		if err != nil {
			return "", 0, err
		}

		switch b[0] {
		case 3: // Ctrl-C
			fmt.Fprintf(e.tty, "\r\n")
			return "", 0, ErrInterrupt

		case 4: // Ctrl-D
			if len(e.buf) == 0 {
				fmt.Fprintf(e.tty, "\r\n")
				return "", 0, io.EOF
			}

		case 13, 10: // Enter
			fmt.Fprintf(e.tty, "\r\n")
			return string(e.buf), e.pos, nil

		case 127, 8: // Backspace / Ctrl-H
			if e.pos > 0 {
				_, size := prevRune(e.buf, e.pos)
				copy(e.buf[e.pos-size:], e.buf[e.pos:])
				e.buf = e.buf[:len(e.buf)-size]
				e.pos -= size
			}

		case 1: // Ctrl-A (Home)
			e.pos = 0

		case 5: // Ctrl-E (End)
			e.pos = len(e.buf)

		case 21: // Ctrl-U (clear line)
			e.buf = e.buf[:0]
			e.pos = 0

		case 27: // Escape sequence
			n, _ := e.tty.Read(esc[:1])
			if n == 0 {
				continue
			}
			if esc[0] == '[' {
				n, _ = e.tty.Read(esc[1:2])
				if n == 0 {
					continue
				}
				switch esc[1] {
				case 'D': // Left
					if e.pos > 0 {
						_, size := prevRune(e.buf, e.pos)
						e.pos -= size
					}
				case 'C': // Right
					if e.pos < len(e.buf) {
						_, size := utf8.DecodeRune(e.buf[e.pos:])
						e.pos += size
					}
				case 'H': // Home
					e.pos = 0
				case 'F': // End
					e.pos = len(e.buf)
				case '3': // Delete key: \x1b[3~
					e.tty.Read(esc[2:3]) // consume '~'
					if e.pos < len(e.buf) {
						_, size := utf8.DecodeRune(e.buf[e.pos:])
						copy(e.buf[e.pos:], e.buf[e.pos+size:])
						e.buf = e.buf[:len(e.buf)-size]
					}
				case '1': // Home: \x1b[1~
					e.tty.Read(esc[2:3])
					e.pos = 0
				case '4': // End: \x1b[4~
					e.tty.Read(esc[2:3])
					e.pos = len(e.buf)
				}
			}

		default: // Printable character
			if b[0] >= 32 {
				// Determine full UTF-8 sequence length
				ch := []byte{b[0]}
				if b[0] >= 0xC0 {
					extra := utf8RuneLen(b[0]) - 1
					tmp := make([]byte, extra)
					e.tty.Read(tmp)
					ch = append(ch, tmp...)
				}
				// Insert at cursor position
				e.buf = append(e.buf, make([]byte, len(ch))...)
				copy(e.buf[e.pos+len(ch):], e.buf[e.pos:len(e.buf)-len(ch)])
				copy(e.buf[e.pos:], ch)
				e.pos += len(ch)
			}
		}

		e.redraw(prompt)
	}
}

// redraw clears the current line and redraws prompt + buffer with cursor.
func (e *Editor) redraw(prompt string) {
	// \r = carriage return, \x1b[K = clear to end of line
	fmt.Fprintf(e.tty, "\r\x1b[K%s%s", prompt, string(e.buf))

	// Move cursor back to the correct position
	tailLen := runeCount(e.buf[e.pos:])
	if tailLen > 0 {
		fmt.Fprintf(e.tty, "\x1b[%dD", tailLen)
	}
}

// prevRune returns the rune and byte size of the rune before pos.
func prevRune(buf []byte, pos int) (rune, int) {
	if pos <= 0 {
		return 0, 0
	}
	// Walk back to find the start of the rune
	i := pos - 1
	for i > 0 && !utf8.RuneStart(buf[i]) {
		i--
	}
	r, size := utf8.DecodeRune(buf[i:pos])
	return r, size
}

// runeCount returns the number of runes in b.
func runeCount(b []byte) int {
	return utf8.RuneCount(b)
}

// utf8RuneLen returns the expected byte length of a UTF-8 sequence
// from its leading byte.
func utf8RuneLen(lead byte) int {
	if lead < 0xC0 {
		return 1
	}
	if lead < 0xE0 {
		return 2
	}
	if lead < 0xF0 {
		return 3
	}
	return 4
}

// ErrInterrupt is returned when the user presses Ctrl-C.
var ErrInterrupt = fmt.Errorf("interrupted")
