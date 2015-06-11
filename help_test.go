// Copyright 2012-2015 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package cmd_test

import (
	"strings"

	gitjujutesting "github.com/juju/testing"
	gc "gopkg.in/check.v1"

	"github.com/juju/cmd"
	"github.com/juju/cmd/cmdtesting"
)

type HelpCommandSuite struct {
	gitjujutesting.IsolationSuite
}

var _ = gc.Suite(&HelpCommandSuite{})

func (s *HelpCommandSuite) TestSimple(c *gc.C) {
	jc := cmd.NewSuperCommand(cmd.SuperCommandParams{Name: "jujutest"})
	jc.Register(&TestCommand{Name: "blah"})
	ctx := cmdtesting.Context(c)
	code := cmd.Main(jc, ctx, []string{"blah", "--help"})
	c.Assert(code, gc.Equals, 0)
	stripped := strings.Replace(bufferString(ctx.Stdout), "\n", "", -1)
	c.Assert(stripped, gc.Matches, "usage: jujutest blah.*blah-doc.*")
}

func (s *HelpCommandSuite) TestPrefix(c *gc.C) {
	jc := cmd.NewSuperCommand(cmd.SuperCommandParams{Name: "jujutest", UsagePrefix: "juju"})
	jc.Register(&TestCommand{Name: "blah"})
	ctx := cmdtesting.Context(c)
	code := cmd.Main(jc, ctx, []string{"help"})
	c.Assert(code, gc.Equals, 0)
	stripped := strings.Replace(bufferString(ctx.Stdout), "\n", "", -1)
	c.Assert(stripped, gc.Matches, "usage: juju jujutest <command> ...*")
}

func (s *HelpCommandSuite) TestPrefixFlag(c *gc.C) {
	jc := cmd.NewSuperCommand(cmd.SuperCommandParams{Name: "jujutest", UsagePrefix: "juju"})
	jc.Register(&TestCommand{Name: "blah"})
	ctx := cmdtesting.Context(c)
	code := cmd.Main(jc, ctx, []string{"blah", "--help"})
	c.Assert(code, gc.Equals, 0)
	stripped := strings.Replace(bufferString(ctx.Stdout), "\n", "", -1)
	c.Assert(stripped, gc.Matches, "usage: juju jujutest blah.*blah-doc.*")
}

func (s *HelpCommandSuite) TestPrefixCommand(c *gc.C) {
	jc := cmd.NewSuperCommand(cmd.SuperCommandParams{Name: "jujutest", UsagePrefix: "juju"})
	jc.Register(&TestCommand{Name: "blah"})
	ctx := cmdtesting.Context(c)
	code := cmd.Main(jc, ctx, []string{"help", "blah"})
	c.Assert(code, gc.Equals, 0)
	stripped := strings.Replace(bufferString(ctx.Stdout), "\n", "", -1)
	c.Assert(stripped, gc.Matches, "usage: juju jujutest blah.*blah-doc.*")
}

func (s *HelpCommandSuite) TestMultipleSuperCommands(c *gc.C) {
	level1 := cmd.NewSuperCommand(cmd.SuperCommandParams{Name: "level1"})
	level2 := cmd.NewSuperCommand(cmd.SuperCommandParams{Name: "level2", UsagePrefix: "level1"})
	level1.Register(level2)
	level3 := cmd.NewSuperCommand(cmd.SuperCommandParams{Name: "level3", UsagePrefix: "level1 level2"})
	level2.Register(level3)
	level3.Register(&TestCommand{Name: "blah"})
	ctx := cmdtesting.Context(c)
	code := cmd.Main(level1, ctx, []string{"help", "level2", "level3", "blah"})
	c.Assert(code, gc.Equals, 0)
	stripped := strings.Replace(bufferString(ctx.Stdout), "\n", "", -1)
	c.Assert(stripped, gc.Matches, "usage: level1 level2 level3 blah.*blah-doc.*")
}
