// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	gc "gopkg.in/check.v1"

	"github.com/juju/cmd"
	"github.com/juju/cmd/cmdtesting"
)

type CmdSuite struct{}

var _ = gc.Suite(&CmdSuite{})

func (s *CmdSuite) TestContext(c *gc.C) {
	ctx := cmdtesting.Context(c)
	c.Assert(ctx.AbsPath("/foo/bar"), gc.Equals, "/foo/bar")
	c.Assert(ctx.AbsPath("foo/bar"), gc.Equals, filepath.Join(ctx.Dir, "foo/bar"))
}

func (s *CmdSuite) TestContextGetenv(c *gc.C) {
	ctx := cmdtesting.Context(c)
	ctx.Env = make(map[string]string)
	before := ctx.Getenv("foo")
	ctx.Env["foo"] = "bar"
	after := ctx.Getenv("foo")

	c.Check(before, gc.Equals, "")
	c.Check(after, gc.Equals, "bar")
}

func (s *CmdSuite) TestContextSetenv(c *gc.C) {
	ctx := cmdtesting.Context(c)
	before := ctx.Env["foo"]
	ctx.Setenv("foo", "bar")
	after := ctx.Env["foo"]

	c.Check(before, gc.Equals, "")
	c.Check(after, gc.Equals, "bar")
}

func (s *CmdSuite) TestInfo(c *gc.C) {
	minimal := &TestCommand{Name: "verb", Minimal: true}
	help := minimal.Info().Help(cmdtesting.NewFlagSet())
	c.Assert(string(help), gc.Equals, minimalHelp)

	full := &TestCommand{Name: "verb"}
	f := cmdtesting.NewFlagSet()
	var ignored string
	f.StringVar(&ignored, "option", "", "option-doc")
	help = full.Info().Help(f)
	c.Assert(string(help), gc.Equals, fullHelp)

	optionInfo := full.Info()
	optionInfo.Doc = ""
	help = optionInfo.Help(f)
	c.Assert(string(help), gc.Equals, optionHelp)
}

var initErrorTests = []struct {
	c    *TestCommand
	help string
}{
	{&TestCommand{Name: "verb"}, fullHelp},
	{&TestCommand{Name: "verb", Minimal: true}, minimalHelp},
}

func (s *CmdSuite) TestMainInitError(c *gc.C) {
	for _, t := range initErrorTests {
		ctx := cmdtesting.Context(c)
		result := cmd.Main(t.c, ctx, []string{"--unknown"})
		c.Assert(result, gc.Equals, 2)
		c.Assert(bufferString(ctx.Stdout), gc.Equals, "")
		expected := "error: flag provided but not defined: --unknown\n"
		c.Assert(bufferString(ctx.Stderr), gc.Equals, expected)
	}
}

func (s *CmdSuite) TestMainRunError(c *gc.C) {
	ctx := cmdtesting.Context(c)
	result := cmd.Main(&TestCommand{Name: "verb"}, ctx, []string{"--option", "error"})
	c.Assert(result, gc.Equals, 1)
	c.Assert(bufferString(ctx.Stdout), gc.Equals, "")
	c.Assert(bufferString(ctx.Stderr), gc.Equals, "error: BAM!\n")
}

func (s *CmdSuite) TestMainRunSilentError(c *gc.C) {
	ctx := cmdtesting.Context(c)
	result := cmd.Main(&TestCommand{Name: "verb"}, ctx, []string{"--option", "silent-error"})
	c.Assert(result, gc.Equals, 1)
	c.Assert(bufferString(ctx.Stdout), gc.Equals, "")
	c.Assert(bufferString(ctx.Stderr), gc.Equals, "")
}

func (s *CmdSuite) TestMainSuccess(c *gc.C) {
	ctx := cmdtesting.Context(c)
	result := cmd.Main(&TestCommand{Name: "verb"}, ctx, []string{"--option", "success!"})
	c.Assert(result, gc.Equals, 0)
	c.Assert(bufferString(ctx.Stdout), gc.Equals, "success!\n")
	c.Assert(bufferString(ctx.Stderr), gc.Equals, "")
}

func (s *CmdSuite) TestStdin(c *gc.C) {
	const phrase = "Do you, Juju?"
	ctx := cmdtesting.Context(c)
	ctx.Stdin = bytes.NewBuffer([]byte(phrase))
	result := cmd.Main(&TestCommand{Name: "verb"}, ctx, []string{"--option", "echo"})
	c.Assert(result, gc.Equals, 0)
	c.Assert(bufferString(ctx.Stdout), gc.Equals, phrase)
	c.Assert(bufferString(ctx.Stderr), gc.Equals, "")
}

func (s *CmdSuite) TestMainHelp(c *gc.C) {
	for _, arg := range []string{"-h", "--help"} {
		ctx := cmdtesting.Context(c)
		result := cmd.Main(&TestCommand{Name: "verb"}, ctx, []string{arg})
		c.Assert(result, gc.Equals, 0)
		c.Assert(bufferString(ctx.Stdout), gc.Equals, fullHelp)
		c.Assert(bufferString(ctx.Stderr), gc.Equals, "")
	}
}

func (s *CmdSuite) TestDefaultContextReturnsErrorInDeletedDirectory(c *gc.C) {
	ctx := cmdtesting.Context(c)
	wd, err := os.Getwd()
	c.Assert(err, gc.IsNil)
	missing := ctx.Dir + "/missing"
	err = os.Mkdir(missing, 0700)
	c.Assert(err, gc.IsNil)
	err = os.Chdir(missing)
	c.Assert(err, gc.IsNil)
	defer os.Chdir(wd)
	err = os.Remove(missing)
	c.Assert(err, gc.IsNil)
	ctx, err = cmd.DefaultContext()
	c.Assert(err, gc.ErrorMatches, `getwd: no such file or directory`)
	c.Assert(ctx, gc.IsNil)
}

func (s *CmdSuite) TestCheckEmpty(c *gc.C) {
	c.Assert(cmd.CheckEmpty(nil), gc.IsNil)
	c.Assert(cmd.CheckEmpty([]string{"boo!"}), gc.ErrorMatches, `unrecognized args: \["boo!"\]`)
}

func (s *CmdSuite) TestZeroOrOneArgs(c *gc.C) {

	expectValue := func(args []string, expected string) {
		arg, err := cmd.ZeroOrOneArgs(args)
		c.Assert(arg, gc.Equals, expected)
		c.Assert(err, gc.IsNil)
	}

	expectValue(nil, "")
	expectValue([]string{}, "")
	expectValue([]string{"foo"}, "foo")

	arg, err := cmd.ZeroOrOneArgs([]string{"foo", "bar"})
	c.Assert(arg, gc.Equals, "")
	c.Assert(err, gc.ErrorMatches, `unrecognized args: \["bar"\]`)
}

func (s *CmdSuite) TestIsErrSilent(c *gc.C) {
	c.Assert(cmd.IsErrSilent(cmd.ErrSilent), gc.Equals, true)
	c.Assert(cmd.IsErrSilent(cmd.NewRcPassthroughError(99)), gc.Equals, true)
	c.Assert(cmd.IsErrSilent(fmt.Errorf("noisy")), gc.Equals, false)
}
