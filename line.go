/*
Package liner implements a simple command line editor, inspired by linenoise
(https://github.com/antirez/linenoise/). This package supports WIN32 in
addition to the xterm codes supported by everything else.
*/
package liner

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// HistoryLimit is the maximum number of entries saved in the scrollback history.
const HistoryLimit = 100

type action int

const (
	left action = iota
	right
	up
	down
	home
	end
	insert
	del
	pageUp
	pageDown
	f1
	f2
	f3
	f4
	f5
	f6
	f7
	f8
	f9
	f10
	f11
	f12
	unknown
)

type commonState struct {
	history   []string
	supported bool
}

// ReadHistory reads scrollback history from r. Returns the number of lines
// read, and any read error (except io.EOF).
func (s *State) ReadHistory(r io.Reader) (num int, err error) {
	in := bufio.NewReader(r)
	num = 0
	for {
		line, part, err := in.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return num, err
		}
		if part {
			return num, errors.New("Line too long")
		}
		if !utf8.Valid(line) {
			return num, errors.New("Invalid string")
		}
		num++
		s.history = append(s.history, string(line))
		if len(s.history) > HistoryLimit {
			s.history = s.history[1:]
		}
	}
	return num, nil
}

// WriteHistory writes scrollback history to w. Returns the number of lines
// successfully written, and any write error.
func (s *State) WriteHistory(w io.Writer) (num int, err error) {
	for _, item := range s.history {
		_, err := fmt.Fprintln(w, item)
		if err != nil {
			return num, err
		}
		num++
	}
	return num, nil
}

// AppendHistory appends an entry to the scrollback history. AppendHistory
// should be called iff Prompt returns a valid command.
func (s *State) AppendHistory(item string) {
	if len(s.history) > 0 {
		if item == s.history[len(s.history)-1] {
			return
		}
	}
	s.history = append(s.history, item)
	if len(s.history) > HistoryLimit {
		s.history = s.history[1:]
	}
}

const (
	ctrlA = 1
	ctrlB = 2
	ctrlC = 3
	ctrlD = 4
	ctrlE = 5
	ctrlF = 6
	ctrlH = 8
	lf    = 10
	ctrlK = 11
	ctrlL = 12
	cr    = 13
	ctrlN = 14
	ctrlP = 16
	ctrlT = 20
	ctrlU = 21
	esc   = 27
	bs    = 127
)

const (
	beep = "\a"
)

func (s *State) refresh(prompt string, buf string, pos int) error {
	s.cursorPos(0)
	_, err := fmt.Print(prompt)
	if err != nil {
		return err
	}
	_, err = fmt.Print(buf)
	s.eraseLine()
	s.cursorPos(utf8.RuneCountInString(prompt) + pos)
	return err
}

// Prompt displays p, and then waits for user input. Prompt allows line editing
// if the terminal supports it.
func (s *State) Prompt(p string) (string, error) {
	if !s.supported {
		return s.promptUnsupported(p)
	}

	fmt.Print(p)
	line := make([]rune, 0)
	pos := 0
	historyPos := len(s.history)
	var historyEnd string

mainLoop:
	for {
		next, err := s.readNext()
		if err != nil {
			return "", err
		}
		switch v := next.(type) {
		case rune:
			switch v {
			case cr, lf:
				break mainLoop
			case ctrlA: // Start of line
				pos = 0
				s.refresh(p, string(line), pos)
			case ctrlE: // End of line
				pos = len(line)
				s.refresh(p, string(line), pos)
			case ctrlB: // left
				if pos > 0 {
					pos--
					s.refresh(p, string(line), pos)
				} else {
					fmt.Print(beep)
				}
			case ctrlF: // right
				if pos < len(line) {
					pos++
					s.refresh(p, string(line), pos)
				} else {
					fmt.Print(beep)
				}
			case ctrlD: // del
				if pos >= len(line) {
					fmt.Print(beep)
				} else {
					line = append(line[:pos], line[pos+1:]...)
					s.refresh(p, string(line), pos)
				}
			case ctrlK: // delete remainder of line
				if pos >= len(line) {
					fmt.Print(beep)
				} else {
					line = line[:pos]
					s.refresh(p, string(line), pos)
				}
			case ctrlP: // up
				if historyPos > 0 {
					if historyPos == len(s.history) {
						historyEnd = string(line)
					}
					historyPos--
					line = []rune(s.history[historyPos])
					pos = len(line)
					s.refresh(p, string(line), pos)
				} else {
					fmt.Print(beep)
				}
			case ctrlN: // down
				if historyPos < len(s.history) {
					historyPos++
					if historyPos == len(s.history) {
						line = []rune(historyEnd)
					} else {
						line = []rune(s.history[historyPos])
					}
					pos = len(line)
					s.refresh(p, string(line), pos)
				} else {
					fmt.Print(beep)
				}
			case ctrlT: // transpose prev rune with rune under cursor
				if len(line) < 2 || pos < 1 {
					fmt.Print(beep)
				} else {
					if pos == len(line) {
						pos--
					}
					line[pos-1], line[pos] = line[pos], line[pos-1]
					pos++
					s.refresh(p, string(line), pos)
				}
			case ctrlL: // clear screen
				s.eraseScreen()
				s.refresh(p, string(line), pos)
			case ctrlH, bs: // Backspace
				if pos <= 0 {
					fmt.Print(beep)
				} else {
					line = append(line[:pos-1], line[pos:]...)
					pos--
					s.refresh(p, string(line), pos)
				}
			case ctrlU: // Erase entire line
				line = line[:0]
				pos = 0
				s.refresh(p, string(line), pos)
			// Catch unhandled control codes (anything <= 31)
			case 0, 3, 7, 9, 15:
				fallthrough
			case 17, 18, 19, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31:
				fmt.Print(beep)
			default:
				if pos == len(line) {
					line = append(line, v)
					fmt.Printf("%c", v)
					pos++
				} else {
					line = append(line[:pos], append([]rune{v}, line[pos:]...)...)
					pos++
					s.refresh(p, string(line), pos)
				}
			}
		case action:
			switch v {
			case del:
				if pos >= len(line) {
					fmt.Print(beep)
				} else {
					line = append(line[:pos], line[pos+1:]...)
				}
			case left:
				if pos > 0 {
					pos--
				} else {
					fmt.Print(beep)
				}
			case right:
				if pos < len(line) {
					pos++
				} else {
					fmt.Print(beep)
				}
			case up:
				if historyPos > 0 {
					if historyPos == len(s.history) {
						historyEnd = string(line)
					}
					historyPos--
					line = []rune(s.history[historyPos])
					pos = len(line)
				} else {
					fmt.Print(beep)
				}
			case down:
				if historyPos < len(s.history) {
					historyPos++
					if historyPos == len(s.history) {
						line = []rune(historyEnd)
					} else {
						line = []rune(s.history[historyPos])
					}
					pos = len(line)
				} else {
					fmt.Print(beep)
				}
			}
			s.refresh(p, string(line), pos)
		}
	}
	return string(line), nil
}
