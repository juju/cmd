package cmd_test

import (
	"fmt"

	"github.com/juju/gnuflag"
	gc "gopkg.in/check.v1"

	"github.com/juju/cmd/v3"
)

type documentationSuite struct{}

var _ = gc.Suite(&documentationSuite{})

func (s *documentationSuite) TestFormatCommand(c *gc.C) {
	tests := []struct {
		command  cmd.Command
		title    bool
		expected string
	}{{
		command: &docTestCommand{
			info: &cmd.Info{
				Name:     "add-cloud",
				Args:     "<cloud name> [<cloud definition file>]",
				Purpose:  "summary for add-cloud...",
				Doc:      "details for add-cloud...",
				Examples: "examples for add-cloud...",
				SeeAlso:  []string{"clouds", "update-cloud", "remove-cloud", "update-credential"},
				Aliases:  []string{"cloud-add", "import-cloud"},
			},
			flags: []string{"force", "format", "output"},
		},
		title: false,
		expected: (`
> See also: [clouds](#clouds), [update-cloud](#update-cloud), [remove-cloud](#remove-cloud), [update-credential](#update-credential)

## Summary
summary for add-cloud...

## Usage
` + "```" + `juju [options] <cloud name> [<cloud definition file>]` + "```" + `

### Options
| Flag | Default | Usage |
| --- | --- | --- |
| ` + "`" + `--force` + "`" + ` | default value for "force" flag | description for "force" flag |
| ` + "`" + `--format` + "`" + ` | default value for "format" flag | description for "format" flag |
| ` + "`" + `--output` + "`" + ` | default value for "output" flag | description for "output" flag |

## Examples
examples for add-cloud...

## Details
details for add-cloud...

---

`)[1:],
	}}

	for _, t := range tests {
		output := cmd.FormatCommand(
			t.command,
			&cmd.SuperCommand{Name: "juju"},
			t.title,
		)
		fmt.Println("------------")
		fmt.Println(output)
		fmt.Println("------------")
		c.Check(output, gc.Equals, t.expected)
	}
}

// docTestCommand is a fake implementation of cmd.Command, used for testing
// documentation output.
type docTestCommand struct {
	info  *cmd.Info
	flags []string
}

func (c *docTestCommand) Info() *cmd.Info {
	return c.info
}

func (c *docTestCommand) SetFlags(f *gnuflag.FlagSet) {
	for _, flag := range c.flags {
		f.String(flag,
			fmt.Sprintf("default value for %q flag", flag),
			fmt.Sprintf("description for %q flag", flag))
	}
}

func (c *docTestCommand) IsSuperCommand() bool         { return false }
func (c *docTestCommand) Init(args []string) error     { return nil }
func (c *docTestCommand) Run(ctx *cmd.Context) error   { return nil }
func (c *docTestCommand) AllowInterspersedFlags() bool { return false }
