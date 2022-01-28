// not thread safe, use multiple adb clients if needed.
package adb

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
)

type CmdError struct{ ExitCode int }

func (e *CmdError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("exit code %d", e.ExitCode)
}

const delim = "[ADB\x01CMD\x01DONE]"

var delimb = []byte(delim)
var deliml = len(delimb)

type ADB struct {
	bin       string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *output
	stderr    *output
	buf       []byte
	maxBuffer int
}

func New(executable string, maxBuffer int) *ADB {
	return &ADB{bin: executable, buf: make([]byte, 1024*4), maxBuffer: maxBuffer}
}

func (adb *ADB) Init() error {
	cmd := exec.Command(adb.bin, "shell")
	cmd.SysProcAttr = parentlessSysProc()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	adb.stdin = stdin
	adb.stdout = newOutput(stdout, adb.maxBuffer)
	adb.stderr = newOutput(stderr, adb.maxBuffer)
	adb.cmd = cmd
	return nil
}

func (adb *ADB) Close() error {
	if adb.cmd == nil {
		return nil
	}

	adb.stdin.Close()
	err := adb.cmd.Wait()
	adb.cmd = nil
	return err
}

// Run a command and pipe output to their respective writers.
func (adb *ADB) Run(cmd string, stdout, stderr io.Writer) error {
	done := make(chan error, 1)
	go func() {
		done <- func() (err error) {
			if _, err = fmt.Fprintln(adb.stdin, cmd); err != nil {
				return err
			}
			if _, err = fmt.Fprintf(adb.stdin, "printf '%%03d' $?; echo -en '%s'\n", delim); err != nil {
				return err
			}
			if _, err = fmt.Fprintf(adb.stdin, "echo -en '%s' >&2\n", delim); err != nil {
				return err
			}
			return nil
		}()
	}()

	go func() {
		done <- func() error {
			d, err := adb.stdout.Next()
			if err != nil {
				return err
			}
			if len(d) < 3 {
				return io.EOF
			}
			exit, err := strconv.Atoi(string(d[len(d)-3:]))
			d = d[:len(d)-3]
			if err != nil {
				return fmt.Errorf("something went wrong: could not parse exit code")
			}

			var exitErr error
			if exit != 0 {
				exitErr = &CmdError{exit}
			}

			if stdout == nil {
				return exitErr
			}

			if _, err = stdout.Write(d); err != nil {
				return err
			}

			return exitErr
		}()
	}()

	go func() {
		done <- func() error {
			d, err := adb.stderr.Next()
			if err != nil {
				return err
			}

			if stderr == nil {
				return nil
			}
			_, err = stderr.Write(d)
			return err
		}()
	}()

	var err error
	for i := 0; i < 3; i++ {
		if e := <-done; e != nil && err == nil {
			err = e
		}
	}

	var ce *CmdError
	if err != nil && !errors.As(err, &ce) {
		_ = adb.Close()
		_ = adb.Init()
	}

	return err
}

type output struct {
	io.Reader
	*bufio.Scanner
}

func newOutput(r io.Reader, max int) *output {
	s := bufio.NewScanner(r)
	s.Split(scanDelim)
	s.Buffer(make([]byte, 0, max), max)
	return &output{Reader: r, Scanner: s}
}

func (o *output) Next() ([]byte, error) {
	var b []byte
	if o.Scan() {
		b = o.Bytes()
	}

	return b, o.Err()
}

func scanDelim(data []byte, atEOF bool) (advance int, token []byte, err error) {
	l := len(data)
	if atEOF && l == 0 {
		return 0, nil, nil
	}

	if i := bytes.Index(data, delimb); i >= 0 {
		return i + deliml, data[0:i], nil
	}

	if atEOF {
		return l, data, nil
	}

	return 0, nil, nil
}
