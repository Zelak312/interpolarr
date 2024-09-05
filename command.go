package main

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
)

type Command struct {
	cmd        *exec.Cmd
	name       string
	output     bytes.Buffer
	stdoutPipe io.ReadCloser
	stderrPipe io.ReadCloser
}

func NewCommandContext(ctx context.Context, cmd_name string, args ...string) *Command {
	cmd := exec.CommandContext(ctx, cmd_name, args...)

	command := Command{cmd: cmd, name: cmd_name + " " + strings.Join(args, " ")}
	return &command
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

	multiWriter := io.MultiWriter()
	go func() {
		io.Copy(multiWriter, stdoutPipe)
	}()
	go func() {
		io.Copy(multiWriter, stderrPipe)
	}()

	c.stdoutPipe = stdoutPipe
	c.stderrPipe = stderrPipe

	err = c.cmd.Run()
	return c.output.String(), err
}
