// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"bytes"
	"fmt"

	gc "gopkg.in/check.v1"
)

type VersionSuite struct{}

var _ = gc.Suite(&VersionSuite{})

func (s *VersionSuite) TestVersion(c *gc.C) {
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	const version = "999.888.777"
	code := Main(newVersionCommand(version), ctx, nil)
	c.Check(code, gc.Equals, 0)
	c.Assert(stderr.String(), gc.Equals, "")
	c.Assert(stdout.String(), gc.Equals, version+"\n")
}

func (s *VersionSuite) TestVersionExtraArgs(c *gc.C) {
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	code := Main(newVersionCommand("xxx"), ctx, []string{"foo"})
	c.Check(code, gc.Equals, 2)
	c.Assert(stdout.String(), gc.Equals, "")
	c.Assert(stderr.String(), gc.Matches, "error: unrecognized args.*\n")
}

func (s *VersionSuite) TestVersionJson(c *gc.C) {
	var stdout, stderr bytes.Buffer
	ctx := &Context{
		Stdout: &stdout,
		Stderr: &stderr,
	}
	const version = "999.888.777"
	code := Main(newVersionCommand(version), ctx, []string{"--format", "json"})
	c.Check(code, gc.Equals, 0)
	c.Assert(stderr.String(), gc.Equals, "")
	c.Assert(stdout.String(), gc.Equals, fmt.Sprintf("%q\n", version))
}
