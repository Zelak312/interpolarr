package main

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
)

type Command struct {
	cmd              *exec.Cmd
	name             string
	output           bytes.Buffer
	stdoutPipe       io.ReadCloser
	stdoutPipeWriter io.WriteCloser
	stderrPipe       io.ReadCloser
	stderrPipeWriter io.WriteCloser
}

func NewCommandContext(ctx context.Context, cmd_name string, args ...string) *Command {
	cmd := exec.CommandContext(ctx, cmd_name, args...)

	c := Command{cmd: cmd, name: cmd_name + " " + strings.Join(args, " ")}

	stdoutPipeReader, stdoutPipeWriter := io.Pipe()
	c.stdoutPipe = stdoutPipeReader
	c.stdoutPipeWriter = stdoutPipeWriter

	stderrPipeReader, stderrPipeWriter := io.Pipe()
	c.stderrPipe = stderrPipeReader
	c.stderrPipeWriter = stderrPipeWriter
	return &c
}

func CreateAndRunCommandContext(ctx context.Context, cmd_name string, args ...string) (string, error) {
	cmd := NewCommandContext(ctx, cmd_name, args...)
	return cmd.CombinedOutput()
}

func (c *Command) CombinedOutput() (string, error) {
	stdoutPipe, err := c.cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	stderrPipe, err := c.cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	multiWriterStdout := io.MultiWriter(&c.output, c.stdoutPipeWriter)
	multiWriterStderr := io.MultiWriter(&c.output, c.stderrPipeWriter)
	go func() {
		io.Copy(multiWriterStdout, stdoutPipe)
		c.stdoutPipeWriter.Close()
	}()
	go func() {
		io.Copy(multiWriterStderr, stderrPipe)
		c.stderrPipeWriter.Close()
	}()

	err = c.cmd.Run()
	c.stdoutPipe.Close()
	c.stderrPipe.Close()
	return c.output.String(), err
}
