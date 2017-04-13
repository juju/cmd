// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd_test

import (
	"fmt"

	"github.com/juju/testing"
	gc "gopkg.in/check.v1"

	"github.com/juju/cmd"
	"github.com/juju/cmd/cmdtesting"
)

type VersionSuite struct {
	testing.LoggingSuite
}

var _ = gc.Suite(&VersionSuite{})

func (s *VersionSuite) TestVersion(c *gc.C) {
	const version = "999.888.777"

	ctx := cmdtesting.Context(c)
	code := cmd.Main(cmd.NewVersionCommand(version), ctx, nil)
	c.Check(code, gc.Equals, 0)
	c.Assert(cmdtesting.Stderr(ctx), gc.Equals, "")
	c.Assert(cmdtesting.Stdout(ctx), gc.Equals, version+"\n")
}

func (s *VersionSuite) TestVersionExtraArgs(c *gc.C) {
	ctx := cmdtesting.Context(c)
	code := cmd.Main(cmd.NewVersionCommand("xxx"), ctx, []string{"foo"})
	c.Check(code, gc.Equals, 2)
	c.Assert(cmdtesting.Stdout(ctx), gc.Equals, "")
	c.Assert(cmdtesting.Stderr(ctx), gc.Matches, "ERROR unrecognized args.*\n")
}

func (s *VersionSuite) TestVersionJson(c *gc.C) {
	const version = "999.888.777"

	ctx := cmdtesting.Context(c)
	code := cmd.Main(cmd.NewVersionCommand(version), ctx, []string{"--format", "json"})
	c.Check(code, gc.Equals, 0)
	c.Assert(cmdtesting.Stderr(ctx), gc.Equals, "")
	c.Assert(cmdtesting.Stdout(ctx), gc.Equals, fmt.Sprintf("%q\n", version))
}
