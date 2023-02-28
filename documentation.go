// Copyright 2012-2022 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/juju/gnuflag"
)

const (
	DocumentationFileName      = "documentation.md"
	DocumentationIndexFileName = "index.md"
)

var doc string = `
This command generates a markdown formatted document with all the commands, their descriptions, arguments, and examples.
`

type documentationCommand struct {
	CommandBase
	super   *SuperCommand
	out     string
	noIndex bool
	split   bool
	url     string
}

func newDocumentationCommand(s *SuperCommand) *documentationCommand {
	return &documentationCommand{super: s}
}

func (c *documentationCommand) Info() *Info {
	return &Info{
		Name:    "documentation",
		Args:    "--out <target-folder> --noindex --split --url <base-url>",
		Purpose: "Generate the documentation for all commands",
		Doc:     doc,
	}
}

// SetFlags adds command specific flags to the flag set.
func (c *documentationCommand) SetFlags(f *gnuflag.FlagSet) {
	f.StringVar(&c.out, "out", "", "Documentation output folder if not set the result is displayed using the standard output")
	f.BoolVar(&c.noIndex, "no-index", false, "Do not generate the commands index")
	f.BoolVar(&c.split, "split", false, "Generate one file per command")
	f.StringVar(&c.url, "url", "", "Documentation host URL")
}

func (c *documentationCommand) Run(ctx *Context) error {
	if c.split {
		if c.out == "" {
			return errors.New("set the output folder when using the split option")
		}
		return c.dumpSeveralFiles()
	}
	return c.dumpOneFile(ctx)
}

// dumpeOneFile is invoked when the output is contained in a single output
func (c *documentationCommand) dumpOneFile(ctx *Context) error {
	var writer *bufio.Writer
	if c.out != "" {
		_, err := os.Stat(c.out)
		if err != nil {
			return err
		}

		target := fmt.Sprintf("%s/%s", c.out, DocumentationFileName)

		f, err := os.Create(target)
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

// getSortedListCommands returns an array with the sorted list of
// command names
func (c *documentationCommand) getSortedListCommands() []string {
	// sort the commands
	sorted := make([]string, len(c.super.subcmds))
	i := 0
	for k := range c.super.subcmds {
		sorted[i] = k
		i++
	}
	sort.Strings(sorted)
	return sorted
}

// dumpSeveralFiles is invoked when every command is dumped into
// a separated entity
func (c *documentationCommand) dumpSeveralFiles() error {
	_, err := os.Stat(c.out)
	if err != nil {
		return err
	}

	if len(c.super.subcmds) == 0 {
		fmt.Printf("No commands found for %s", c.super.Name)
		return nil
	}

	sorted := c.getSortedListCommands()

	// create index if indicated
	if !c.noIndex {
		target := fmt.Sprintf("%s/%s", c.out, DocumentationIndexFileName)
		f, err := os.Create(target)
		if err != nil {
			return err
		}

		writer := bufio.NewWriter(f)
		_, err = writer.WriteString(c.commandsIndex(sorted))
		if err != nil {
			return err
		}
		f.Close()
	}

	folder := c.out + "/%s.md"
	for _, command := range sorted {
		target := fmt.Sprintf(folder, command)
		f, err := os.Create(target)
		if err != nil {
			return err
		}
		writer := bufio.NewWriter(f)
		formatted := c.formatCommand(c.super.subcmds[command], false)
		_, err = writer.WriteString(formatted)
		if err != nil {
			return err
		}
		writer.Flush()
		f.Close()
	}

	return err
}

func (c *documentationCommand) dumpEntries(writer *bufio.Writer) error {
	if len(c.super.subcmds) == 0 {
		fmt.Printf("No commands found for %s", c.super.Name)
		return nil
	}

	sorted := c.getSortedListCommands()

	if !c.noIndex {
		_, err := writer.WriteString(c.commandsIndex(sorted))
		if err != nil {
			return err
		}
	}

	var err error
	for _, nameCmd := range sorted {
		_, err = writer.WriteString(c.formatCommand(c.super.subcmds[nameCmd], true))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *documentationCommand) commandsIndex(listCommands []string) string {
	index := "# Index\n"
	prefix := "#"
	if c.url != "" {
		prefix = c.url + "/"
	}
	for id, name := range listCommands {
		index += fmt.Sprintf("%d. [%s](%s%s)\n", id, name, prefix, name)
	}
	index += "---\n\n"
	return index
}

// formatCommand returns a string representation of the information contained
// by a command in Markdown format. The title param can be used to set
// whether the command name should be a title or not. This is particularly
// handy when splitting the commands in different files.
func (c *documentationCommand) formatCommand(ref commandReference, title bool) string {
	formatted := ""
	if title {
		formatted = "# " + strings.ToUpper(ref.name) + "\n"
	}

	if ref.alias != "" {
		formatted += "**Alias:** " + ref.alias + "\n"
	}
	if ref.check != nil && ref.check.Obsolete() {
		formatted += "*This command is deprecated*\n"
	}
	formatted += "\n"

	// Summary
	formatted += "## Summary\n" + ref.command.Info().Purpose + "\n\n"

	// Usage
	if ref.command.Info().Args != "" {
		formatted += "## Usage\n```" + ref.command.Info().Args + "```\n\n"
	}

	// Description
	doc := ref.command.Info().Doc
	if doc != "" {
		formatted += "## Description\n" + ref.command.Info().Doc + "\n\n"
	}

	// Examples
	if len(ref.command.Info().Examples) > 0 {
		formatted += "## Examples\n"
		for _, e := range ref.command.Info().Examples {
			formatted += "`" + e + "`\n"
		}
		formatted += "\n"
	}

	// Options
	formattedFlags := c.formatFlags(ref.command)
	if len(formattedFlags) > 0 {
		formatted += "## Options\n" + formattedFlags + "\n"
	}

	// See Also
	if len(ref.command.Info().SeeAlso) > 0 {
		formatted += "## See Also\n"
		prefix := "#"
		if c.url != "" {
			prefix = c.url + "/"
		}
		for _, s := range ref.command.Info().SeeAlso {
			formatted += fmt.Sprintf("[%s](%s%s)\n", s, prefix, s)
		}
		formatted += "\n"
	}

	formatted += "---\n\n"

	return formatted

}

// formatFlags is an internal formatting solution similar to
// the gnuflag.PrintDefaults. The code is extended here
// to permit additional formatting without modifying the
// gnuflag package.
func (d *documentationCommand) formatFlags(c Command) string {
	flagsAlias := FlagAlias(c, "")
	if flagsAlias == "" {
		// For backward compatibility, the default is 'flag'.
		flagsAlias = "flag"
	}
	f := gnuflag.NewFlagSetWithFlagKnownAs(c.Info().Name, gnuflag.ContinueOnError, flagsAlias)
	c.SetFlags(f)

	// group together all flags for a given value
	flags := make(map[interface{}]flagsByLength)
	f.VisitAll(func(f *gnuflag.Flag) {
		flags[f.Value] = append(flags[f.Value], f)
	})

	// sort the output flags by shortest name for each group.
	var byName flagsByName
	for _, fl := range flags {
		sort.Sort(fl)
		byName = append(byName, fl)
	}
	sort.Sort(byName)

	formatted := "| Flag | Default | Usage |\n"
	formatted += "| --- | --- | --- |\n"
	for _, fs := range byName {
		theFlags := ""
		for i, f := range fs {
			if i > 0 {
				theFlags += ", "
			}
			theFlags += fmt.Sprintf("`--%s`", f.Name)
		}
		formatted += fmt.Sprintf("| %s | %s | %s |\n", theFlags, fs[0].DefValue, fs[0].Usage)
	}
	return formatted
}

// flagsByLength is a slice of flags implementing sort.Interface,
// sorting primarily by the length of the flag, and secondarily
// alphabetically.
type flagsByLength []*gnuflag.Flag

func (f flagsByLength) Less(i, j int) bool {
	s1, s2 := f[i].Name, f[j].Name
	if len(s1) != len(s2) {
		return len(s1) < len(s2)
	}
	return s1 < s2
}
func (f flagsByLength) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}
func (f flagsByLength) Len() int {
	return len(f)
}

// flagsByName is a slice of slices of flags implementing sort.Interface,
// alphabetically sorting by the name of the first flag in each slice.
type flagsByName [][]*gnuflag.Flag

func (f flagsByName) Less(i, j int) bool {
	return f[i][0].Name < f[j][0].Name
}
func (f flagsByName) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}
func (f flagsByName) Len() int {
	return len(f)
}
