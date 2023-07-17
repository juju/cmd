// Copyright 2012-2022 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

var documentationExamples = `
    juju documentation
	juju documentation --split 
	juju documentation --split --no-index --out /tmp/docs
	
	To render markdown documentation using a list of existing
	commands, you can use a file with the following syntax
	
	command1: id1
	command2: id2
	commandN: idN

	For example:

	add-cloud: 1183
	add-secret: 1284
	remove-cloud: 4344

	Then, the urls will be populated using the ids indicated
	in the file above.

	juju documentation --split --no-index --out /tmp/docs --discourse-ids /tmp/docs/myids
`

type documentationCommand struct {
	CommandBase
	super   *SuperCommand
	out     string
	noIndex bool
	split   bool
	url     string
	idsPath string
	// ids is contains a numeric id of every command
	// add-cloud: 1112
	// remove-user: 3333
	// etc...
	ids map[string]string
	// reverseAliases maintains a reverse map of the alias and the
	// targetting command. This is used to find the ids corresponding
	// to a given alias
	reverseAliases map[string]string
}

func newDocumentationCommand(s *SuperCommand) *documentationCommand {
	return &documentationCommand{super: s}
}

func (c *documentationCommand) Info() *Info {
	return &Info{
		Name:     "documentation",
		Args:     "--out <target-folder> --no-index --split --url <base-url> --discourse-ids <filepath>",
		Purpose:  "Generate the documentation for all commands",
		Doc:      doc,
		Examples: documentationExamples,
	}
}

// SetFlags adds command specific flags to the flag set.
func (c *documentationCommand) SetFlags(f *gnuflag.FlagSet) {
	f.StringVar(&c.out, "out", "", "Documentation output folder if not set the result is displayed using the standard output")
	f.BoolVar(&c.noIndex, "no-index", false, "Do not generate the commands index")
	f.BoolVar(&c.split, "split", false, "Generate a separate Markdown file for each command")
	f.StringVar(&c.url, "url", "", "Documentation host URL")
	f.StringVar(&c.idsPath, "discourse-ids", "", "File containing a mapping of commands and their discourse ids")
}

func (c *documentationCommand) Run(ctx *Context) error {
	if c.split {
		if c.out == "" {
			return errors.New("when using --split, you must set the output folder using --out=<folder>")
		}
		return c.dumpSeveralFiles()
	}
	return c.dumpOneFile(ctx)
}

// dumpOneFile is invoked when the output is contained in a single output
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

func (c *documentationCommand) computeReverseAliases() {
	c.reverseAliases = make(map[string]string)

	for name, content := range c.super.subcmds {
		for _, alias := range content.command.Info().Aliases {
			c.reverseAliases[alias] = name
		}
	}

}

// dumpSeveralFiles is invoked when every command is dumped into
// a separated entity
func (c *documentationCommand) dumpSeveralFiles() error {
	if len(c.super.subcmds) == 0 {
		fmt.Printf("No commands found for %s", c.super.Name)
		return nil
	}

	// Attempt to create output directory. This will fail if:
	// - we don't have permission to create the dir
	// - a file already exists at the given path
	err := os.MkdirAll(c.out, os.ModePerm)
	if err != nil {
		return err
	}

	if c.idsPath != "" {
		// get the list of ids
		c.ids, err = c.readFileIds(c.idsPath)
		if err != nil {
			return err
		}
	}

	// create index if indicated
	if !c.noIndex {
		target := fmt.Sprintf("%s/%s", c.out, DocumentationIndexFileName)
		f, err := os.Create(target)
		if err != nil {
			return err
		}

		writer := bufio.NewWriter(f)
		_, err = writer.WriteString(c.commandsIndex())
		if err != nil {
			return err
		}
		f.Close()
	}

	return c.writeDocs(c.out, []string{c.super.Name}, true)
}

// writeDocs (recursively) writes docs for all commands in the given folder.
func (c *documentationCommand) writeDocs(folder string, superCommands []string, printDefaultCommands bool) error {
	c.computeReverseAliases()

	for name, ref := range c.super.subcmds {
		if !printDefaultCommands && isDefaultCommand(name) {
			continue
		}
		commandSeq := append(superCommands, name)
		target := fmt.Sprintf("%s.md", strings.Join(commandSeq[1:], "_"))
		target = strings.ReplaceAll(target, " ", "_")
		target = filepath.Join(folder, target)

		f, err := os.Create(target)
		if err != nil {
			return err
		}
		writer := bufio.NewWriter(f)
		formatted := c.formatCommand(ref, false, commandSeq)
		_, err = writer.WriteString(formatted)
		if err != nil {
			return err
		}
		writer.Flush()
		f.Close()

		// Handle subcommands
		if sc, ok := ref.command.(*SuperCommand); ok {
			err = sc.documentation.writeDocs(folder, commandSeq, false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *documentationCommand) readFileIds(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := bufio.NewScanner(f)
	ids := make(map[string]string)
	for reader.Scan() {
		line := reader.Text()
		items := strings.Split(line, ":")
		if len(items) != 2 {
			return nil, fmt.Errorf("malformed line [%s]", line)
		}
		command := strings.TrimSpace(items[0])
		id := strings.TrimSpace(items[1])
		ids[command] = id
	}
	return ids, nil
}

// TODO: handle subcommands here
func (c *documentationCommand) dumpEntries(writer *bufio.Writer) error {
	if len(c.super.subcmds) == 0 {
		fmt.Printf("No commands found for %s", c.super.Name)
		return nil
	}

	if !c.noIndex {
		_, err := writer.WriteString(c.commandsIndex())
		if err != nil {
			return err
		}
	}

	return c.writeSections(writer, []string{c.super.Name}, true)
}

// writeSections (recursively) writes sections for all commands to the given file.
func (c *documentationCommand) writeSections(writer *bufio.Writer, superCommands []string, printDefaultCommands bool) error {
	sorted := c.getSortedListCommands()
	for _, name := range sorted {
		if !printDefaultCommands && isDefaultCommand(name) {
			continue
		}
		ref := c.super.subcmds[name]
		commandSeq := append(superCommands, name)
		_, err := writer.WriteString(c.formatCommand(ref, true, commandSeq))
		if err != nil {
			return err
		}

		// Handle subcommands
		if sc, ok := ref.command.(*SuperCommand); ok {
			err = sc.documentation.writeSections(writer, commandSeq, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *documentationCommand) commandsIndex() string {
	index := "# Index\n"

	listCommands := c.getSortedListCommands()
	for id, name := range listCommands {
		if isDefaultCommand(name) {
			continue
		}
		index += fmt.Sprintf("%d. [%s](%s)\n", id, name, c.linkForCommand(name))
		// TODO: handle subcommands ??
	}
	index += "---\n\n"
	return index
}

// Return the URL/location for the given command
func (c *documentationCommand) linkForCommand(cmd string) string {
	prefix := "#"
	if c.ids != nil {
		prefix = "/t/"
	}
	if c.url != "" {
		prefix = c.url + "/"
	}

	target, err := c.getTargetCmd(cmd)
	if err != nil {
		fmt.Printf("[ERROR] command [%s] has no id, please add it to the list\n", cmd)
		return ""
	}
	return prefix + target
}

// formatCommand returns a string representation of the information contained
// by a command in Markdown format. The title param can be used to set
// whether the command name should be a title or not. This is particularly
// handy when splitting the commands in different files.
func (c *documentationCommand) formatCommand(ref commandReference, title bool, commandSeq []string) string {
	formatted := ""
	if title {
		formatted = "# " + strings.ToUpper(strings.Join(commandSeq[1:], " ")) + "\n"
	}

	var info *Info
	if ref.name == "documentation" {
		info = c.Info()
	} else {
		info = ref.command.Info()
	}

	// See Also
	if len(info.SeeAlso) > 0 {
		formatted += "> See also: "
		prefix := "#"
		if c.ids != nil {
			prefix = "/t/"
		}
		if c.url != "" {
			prefix = c.url + "t/"
		}

		for i, s := range info.SeeAlso {
			target, err := c.getTargetCmd(s)
			if err != nil {
				fmt.Println(err.Error())
			}
			formatted += fmt.Sprintf("[%s](%s%s)", s, prefix, target)
			if i < len(info.SeeAlso)-1 {
				formatted += ", "
			}
		}
		formatted += "\n"
	}

	if ref.alias != "" {
		formatted += "**Alias:** " + ref.alias + "\n"
	}
	if ref.check != nil && ref.check.Obsolete() {
		formatted += "*This command is deprecated*\n"
	}
	formatted += "\n"

	// Summary
	formatted += "## Summary\n" + info.Purpose + "\n\n"

	// Usage
	if strings.TrimSpace(info.Args) != "" {
		formatted += fmt.Sprintf(`## Usage
`+"```"+`%s [options] %s`+"```"+`

`, strings.Join(commandSeq, " "), info.Args)
	}

	// Options
	formattedFlags := c.formatFlags(ref.command, info)
	if len(formattedFlags) > 0 {
		formatted += "### Options\n" + formattedFlags + "\n"
	}

	// Examples
	examples := info.Examples
	if strings.TrimSpace(examples) != "" {
		formatted += "## Examples\n" + examples + "\n\n"
	}

	// Details
	doc := EscapeMarkdown(info.Doc)
	if strings.TrimSpace(doc) != "" {
		formatted += "## Details\n" + doc + "\n\n"
	}

	formatted += c.formatSubcommands(info.Subcommands, commandSeq)
	formatted += "---\n\n"

	return formatted
}

// getTargetCmd is an auxiliary function that returns the target command or
// the corresponding id if available.
func (d *documentationCommand) getTargetCmd(cmd string) (string, error) {
	// no ids were set, return the original command
	if d.ids == nil {
		return cmd, nil
	}
	target, found := d.ids[cmd]
	if found {
		return target, nil
	} else {
		// check if this is an alias
		targetCmd, found := d.reverseAliases[cmd]
		fmt.Printf("use alias %s -> %s\n", cmd, targetCmd)
		if !found {
			// if we're working with ids, and we have to mmake the translation,
			// we need to have an id per every requested command
			return "", fmt.Errorf("requested id for command %s was not found", cmd)
		}
		return targetCmd, nil

	}
}

// formatFlags is an internal formatting solution similar to
// the gnuflag.PrintDefaults. The code is extended here
// to permit additional formatting without modifying the
// gnuflag package.
func (d *documentationCommand) formatFlags(c Command, info *Info) string {
	flagsAlias := FlagAlias(c, "")
	if flagsAlias == "" {
		// For backward compatibility, the default is 'flag'.
		flagsAlias = "flag"
	}
	f := gnuflag.NewFlagSetWithFlagKnownAs(info.Name, gnuflag.ContinueOnError, flagsAlias)

	// if we are working with the documentation command,
	// we have to set flags on a new instance, otherwise
	// we will overwrite the current flag values
	if info.Name != "documentation" {
		c.SetFlags(f)
	} else {
		c = newDocumentationCommand(d.super)
		c.SetFlags(f)
	}

	// group together all flags for a given value
	flags := make(map[interface{}]flagsByLength)
	f.VisitAll(func(f *gnuflag.Flag) {
		flags[f.Value] = append(flags[f.Value], f)
	})
	if len(flags) == 0 {
		return ""
	}

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
		formatted += fmt.Sprintf("| %s | %s | %s |\n", theFlags,
			EscapeMarkdown(fs[0].DefValue), EscapeMarkdown(fs[0].Usage))
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

// EscapeMarkdown returns a copy of the input string, in which any special
// Markdown characters (e.g. < > |) are escaped.
func EscapeMarkdown(raw string) string {
	escapeSeqs := map[rune]string{
		'<': "&lt;",
		'>': "&gt;",
		'&': "&amp;",
		'|': "&#x7c;",
	}

	var escaped strings.Builder
	escaped.Grow(len(raw))

	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "    ") {
			// Literal code block - don't escape anything
			escaped.WriteString(line)

		} else {
			// Keep track of whether we are inside a code span `...`
			// If so, don't escape characters
			insideCodeSpan := false

			for _, c := range line {
				if c == '`' {
					insideCodeSpan = !insideCodeSpan
				}

				if !insideCodeSpan {
					if escapeSeq, ok := escapeSeqs[c]; ok {
						escaped.WriteString(escapeSeq)
						continue
					}
				}
				escaped.WriteRune(c)
			}
		}

		if i < len(lines)-1 {
			escaped.WriteRune('\n')
		}
	}

	return escaped.String()
}

func (c *documentationCommand) formatSubcommands(subcommands map[string]string, commandSeq []string) string {
	var output string

	sorted := []string{}
	for name := range subcommands {
		if isDefaultCommand(name) {
			continue
		}
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	if len(sorted) > 0 {
		output += "## Subcommands\n"
		for _, name := range sorted {
			output += fmt.Sprintf("- [%s](%s)\n", name,
				c.linkForCommand(strings.Join(append(commandSeq[1:], name), "_")))
		}
		output += "\n"
	}

	return output
}
