// Copyright 2024 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package cmd_test

import (
	"bytes"
	"errors"
	"os"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/cmd/v3"
)

type markdownSuite struct{}

var _ = gc.Suite(&markdownSuite{})

// TestWriteError ensures that the cmd.PrintMarkdown function surfaces errors
// returned by the writer.
func (*markdownSuite) TestWriteError(c *gc.C) {
	expectedErr := errors.New("foo")
	writer := errorWriter{err: expectedErr}
	command := &docTestCommand{
		info: &cmd.Info{},
	}
	err := cmd.PrintMarkdown(writer, command, cmd.MarkdownOptions{})
	c.Assert(err, gc.NotNil)
	c.Check(err, gc.ErrorMatches, ".*foo")
}

// errorWriter is an io.Writer that returns an error whenever the Write method
// is called.
type errorWriter struct {
	err error
}

func (e errorWriter) Write([]byte) (n int, err error) {
	return 0, e.err
}

// TestOutput tests that the output of the PrintMarkdown function is
// fundamentally correct.
func (*markdownSuite) TestOutput(c *gc.C) {
	seeAlso := []string{"clouds", "update-cloud", "remove-cloud", "update-credential"}
	subcommands := map[string]string{
		"foo": "foo the bar baz",
		"bar": "bar the baz foo",
		"baz": "baz the foo bar",
	}

	command := &docTestCommand{
		info: &cmd.Info{
			Name:        "add-cloud",
			Args:        "<cloud name> [<cloud definition file>]",
			Purpose:     "Add a cloud definition to Juju.",
			Doc:         "details for add-cloud...",
			Examples:    "examples for add-cloud...",
			SeeAlso:     seeAlso,
			Aliases:     []string{"new-cloud", "cloud-add"},
			Subcommands: subcommands,
		},
		flags: []testFlag{{
			name: "force",
		}, {
			name:  "file",
			short: "f",
		}, {
			name:  "credential",
			short: "c",
		}},
	}

	// These functions verify the provided argument is in the expected set.
	linkForCommand := func(s string) string {
		for _, cmd := range seeAlso {
			if cmd == s {
				return "https://docs.com/" + cmd
			}
		}
		c.Fatalf("linkForCommand called with unexpected command %q", s)
		return ""
	}

	linkForSubcommand := func(s string) string {
		_, ok := subcommands[s]
		if !ok {
			c.Fatalf("linkForSubcommand called with unexpected subcommand %q", s)
		}
		return "https://docs.com/add-cloud/" + s
	}

	expected, err := os.ReadFile("testdata/add-cloud.md")
	c.Assert(err, jc.ErrorIsNil)

	var buf bytes.Buffer
	err = cmd.PrintMarkdown(&buf, command, cmd.MarkdownOptions{
		Title:             `Command "juju add-cloud"`,
		UsagePrefix:       "juju ",
		LinkForCommand:    linkForCommand,
		LinkForSubcommand: linkForSubcommand,
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Check(buf.String(), gc.Equals, string(expected))
}

// TestOutputWithoutArgs tests that the output of the PrintMarkdown function is
// correct when a command does not need arguments, e.g. list commands.
func (*markdownSuite) TestOutputWithoutArgs(c *gc.C) {
	seeAlso := []string{"add-cloud", "update-cloud", "remove-cloud", "update-credential"}
	subcommands := map[string]string{
		"foo": "foo the bar baz",
		"bar": "bar the baz foo",
		"baz": "baz the foo bar",
	}

	command := &docTestCommand{
		info: &cmd.Info{
			Name:        "clouds",
			Args:        "", //Empty args should still result in a usage field.
			Purpose:     "List clouds.",
			Doc:         "details for clouds...",
			Examples:    "examples for clouds...",
			SeeAlso:     seeAlso,
			Aliases:     []string{"list-clouds"},
			Subcommands: subcommands,
		},
	}

	// These functions verify the provided argument is in the expected set.
	linkForCommand := func(s string) string {
		for _, cmd := range seeAlso {
			if cmd == s {
				return "https://docs.com/" + cmd
			}
		}
		c.Fatalf("linkForCommand called with unexpected command %q", s)
		return ""
	}

	linkForSubcommand := func(s string) string {
		_, ok := subcommands[s]
		if !ok {
			c.Fatalf("linkForSubcommand called with unexpected subcommand %q", s)
		}
		return "https://docs.com/clouds/" + s
	}

	expected, err := os.ReadFile("testdata/list-clouds.md")
	c.Assert(err, jc.ErrorIsNil)

	var buf bytes.Buffer
	err = cmd.PrintMarkdown(&buf, command, cmd.MarkdownOptions{
		Title:             `Command "juju clouds"`,
		UsagePrefix:       "juju ",
		LinkForCommand:    linkForCommand,
		LinkForSubcommand: linkForSubcommand,
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Check(buf.String(), gc.Equals, string(expected))
}
