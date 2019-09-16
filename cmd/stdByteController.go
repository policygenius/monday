package main

import (
	"io"
	"log"
	"bytes"
	"os"
	"sync"
	"os/exec"
	"strings"
)

//buffer writer struct
type byteWriter struct {
	buf bytes.Buffer
	w io.Writer
}

//create new buffer writer
func NewByteWriter(w io.Writer) *byteWriter {
	return &byteWriter{
		w: w,
	}
}

//write buffered bytes
func (w *byteWriter) Write(d []byte) (int, error) {
	w.buf.Write(d)
	return w.w.Write(d)
}

//get the bytes yummy
func (w *byteWriter) Bytes() []byte {
	return w.buf.Bytes()
}

func execrunner(execC string, commArgs []string, wrkdir string) {
	cmd := exec.Command(execC, commArgs...)

	dir, _ := os.Getwd()
	wkdir := strings.Replace(dir, "bifrost", wrkdir, -1)
	cmd.Dir = wkdir

	var errStdout, errStderr error
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()
	stdout := NewByteWriter(os.Stdout)
	stderr := NewByteWriter(os.Stderr)

	err := cmd.Start()
	if err != nil {
		log.Fatalf("Unable to run command: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
		wg.Done()
	}()

	_, errStderr = io.Copy(stderr, stderrIn)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		log.Fatalf("Unable to run command")
	}
	if errStdout != nil || errStderr != nil {
		log.Fatalf("internal failure with std pipe")
	}
}