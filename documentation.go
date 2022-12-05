// Copyright 2012-2022 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/juju/gnuflag"
)

var doc string = `
This command generates a markdown formatted document with all the commands, their descriptions, arguments, and examples.
`

type documentationCommand struct {
	CommandBase
	super   *SuperCommand
	out     string
	noIndex bool
}

func newDocumentationCommand(s *SuperCommand) *documentationCommand {
	return &documentationCommand{super: s}
}

func (c *documentationCommand) Info() *Info {
	return &Info{
		Name:    "documentation",
		Args:    "--out <target-file> --noindex",
		Purpose: "Generate the documentation for current commands",
		Doc:     doc,
	}
}

// SetFlags adds command specific flags to the flag set.
func (c *documentationCommand) SetFlags(f *gnuflag.FlagSet) {
	f.StringVar(&c.out, "out", "", "Documentation output file")
	f.BoolVar(&c.noIndex, "noindex", false, "Do not generate the commands index")
}

func (c *documentationCommand) Run(ctx *Context) error {
	var writer *bufio.Writer
	if c.out != "" {
		f, err := os.Create(c.out)
		if err != nil {
			return err
		}
		defer f.Close()
		writer = bufio.NewWriter(f)
	} else {
		writer = bufio.NewWriter(ctx.Stdout)
	}
	return c.dumpEntries(writer)
}

func (c *documentationCommand) dumpEntries(writer *bufio.Writer) error {
	if len(c.super.subcmds) == 0 {
		return nil
	}

	// sort the commands
	sorted := make([]string, len(c.super.subcmds))
	i := 0
	for k := range c.super.subcmds {
		sorted[i] = k
		i++
	}
	sort.Strings(sorted)

	if !c.noIndex {
		_, err := writer.WriteString(c.commandsIndex(sorted))
		if err != nil {
			return err
		}
	}

	var err error
	for _, nameCmd := range sorted {
		_, err = writer.WriteString(c.formatCommand(c.super.subcmds[nameCmd]))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *documentationCommand) commandsIndex(listCommands []string) string {
	index := "# Index\n"
	for id, name := range listCommands {
		index += fmt.Sprintf("%d. [%s](#%s)\n", id, name, name)
	}
	index += "---\n\n"
	return index
}

func (c *documentationCommand) formatCommand(ref commandReference) string {
	formatted := "# " + strings.ToUpper(ref.name) + "\n"
	if ref.alias != "" {
		formatted += "**Alias:** " + ref.alias + "\n"
	}
	if ref.check != nil && ref.check.Obsolete() {
		formatted += "*This command is deprecated*\n"
	}
	formatted += "\n"

	// Description
	formatted += "## Summary\n" + ref.command.Info().Purpose + "\n\n"

	// Arguments
	if ref.command.Info().Args != "" {
		formatted += "## Arguments\n" + ref.command.Info().Args + "\n\n"
	}

	// Description
	doc := ref.command.Info().Doc
	if doc != "" {
		formatted += "## Description\n" + ref.command.Info().Doc + "\n"
	}

	formatted += "---\n"

	return formatted

}
