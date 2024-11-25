package main

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
)

type Command struct {
	cmd                    *exec.Cmd
	name                   string
	output                 bytes.Buffer
	stdin                  io.WriteCloser
	stdout                 io.ReadCloser
	stderr                 io.ReadCloser
	isOutputBufferDisabled bool
}

func NewCommandContext(ctx context.Context, cmd_name string, args ...string) *Command {
	cmd := exec.CommandContext(ctx, cmd_name, args...)

	c := Command{cmd: cmd, name: cmd_name + " " + strings.Join(args, " "), isOutputBufferDisabled: false}
	return &c
}

func (c *Command) DisableOutputBuffer() {
	c.isOutputBufferDisabled = true
}

func (c *Command) GetStdin() (io.WriteCloser, error) {
	if c.stdin == nil {
		stdin, err := c.cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		c.stdin = stdin
	}
	return c.stdin, nil
}

func (c *Command) GetStdout() (io.ReadCloser, error) {
	if c.stdout == nil {
		stdout, err := c.cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		c.stdout = stdout
	}
	return c.stdout, nil
}

func (c *Command) GetStderr() (io.ReadCloser, error) {
	if c.stderr == nil {
		stderr, err := c.cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		c.stderr = stderr
	}
	return c.stderr, nil
}

func (c *Command) Start() error {
	if !c.isOutputBufferDisabled {
		c.cmd.Stdout = &c.output
		c.cmd.Stderr = &c.output
	}

	return c.cmd.Start()
}

func (c *Command) Wait() error {
	err := c.cmd.Wait()
	return err
}

func (c *Command) CombinedOutput() (string, error) {
	if err := c.Start(); err != nil {
		return "", err
	}

	err := c.Wait()
	return c.GetOutput(), err
}

func (c *Command) GetOutput() string {
	return c.output.String()
}
