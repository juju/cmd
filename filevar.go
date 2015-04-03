// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"errors"
	"io/ioutil"

	"github.com/juju/utils"
)

// FileVar represents a path to a file.
type FileVar struct {
	Path string
}

var ErrNoPath = errors.New("path not set")

// Set stores the chosen path name in f.Path.
func (f *FileVar) Set(v string) error {
	f.Path = v
	return nil
}

// Read returns the contents of the file.
func (f *FileVar) Read(ctx *Context) ([]byte, error) {
	if f.Path == "" {
		return nil, ErrNoPath
	}
	if f.Path == "-" {
		return ioutil.ReadAll(ctx.Stdin)
	}

	path, err := utils.NormalizePath(f.Path)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(ctx.AbsPath(path))
}

// String returns the path to the file.
func (f *FileVar) String() string {
	return f.Path
}
