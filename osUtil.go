package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
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

func FileExist(f string) (bool, error) {
	var err error
	if _, err = os.Stat(f); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, nil
	}

	return true, err
}
