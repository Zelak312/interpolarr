package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

func CopyFile(src string, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

func IsSamePath(p1 string, p2 string) (bool, error) {
	absPath1, err := filepath.Abs(p1)
	if err != nil {
		return false, err
	}

	absPath2, err := filepath.Abs(p2)
	if err != nil {
		return false, err
	}

	// Compare the absolute paths
	return absPath1 == absPath2, nil
}

type Command struct {
	cmd    *exec.Cmd
	name   string
	output bytes.Buffer
}

func (c *Command) Write(p []byte) (n int, err error) {
	log.WithField("cmdName", c.name).Debug(string(p))
	return c.output.Write(p)
}

func (c *Command) CombinedOutput() (string, error) {
	err := c.cmd.Run()
	return c.output.String(), err
}

func CommandContextLogger(ctx context.Context, name string, arg ...string) *Command {
	cmd := exec.CommandContext(ctx, name, arg...)

	command := Command{cmd: cmd, name: name + " " + strings.Join(arg, " ")}
	cmd.Stdout = &command
	cmd.Stderr = &command

	return &command
}
