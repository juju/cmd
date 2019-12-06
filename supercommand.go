// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/juju/errors"
	"github.com/juju/gnuflag"
	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("cmd")

type topic struct {
	short string
	long  func() string
	// Help aliases are not output when topics are listed, but are used
	// to search for the help topic
	alias bool
}

// UnrecognizedCommand defines an error that specifies when a command is not
// found.
type UnrecognizedCommand struct {
	message string
}

// UnrecognizedCommandf creates a UnrecognizedCommand with additional arguments
// to create a bespoke message for the unrecognized command.
func UnrecognizedCommandf(format string, args ...interface{}) *UnrecognizedCommand {
	return &UnrecognizedCommand{
		message: fmt.Sprintf(format, args...),
	}
}

// DefaultUnrecognizedCommand creates a default message for using the
// UnrecognizedCommand.
func DefaultUnrecognizedCommand(name string) *UnrecognizedCommand {
	return UnrecognizedCommandf("unrecognized command: %s", name)
}

func (e *UnrecognizedCommand) Error() string {
	return e.message
}

// MissingCallback defines a function that will be used by the SuperCommand if
// the requested subcommand isn't found.
type MissingCallback func(ctx *Context, subcommand string, args []string) error

// SuperCommandParams provides a way to have default parameter to the
// `NewSuperCommand` call.
type SuperCommandParams struct {
	// UsagePrefix should be set when the SuperCommand is
	// actually a subcommand of some other SuperCommand;
	// if NotifyRun is called, it name will be prefixed accordingly,
	// unless UsagePrefix is identical to Name.
	UsagePrefix string

	// Notify, if not nil, is called when the SuperCommand
	// is about to run a sub-command.
	NotifyRun func(cmdName string)

	// NotifyHelp is called just before help is printed, with the
	// arguments received by the help command. This can be
	// used, for example, to load command information for external
	// "plugin" commands, so that their documentation will show up
	// in the help output.
	NotifyHelp func([]string)

	Name    string
	Purpose string
	Doc     string
	// Log holds the Log value associated with the supercommand. If it's nil,
	// no logging flags will be configured.
	Log *Log
	// GlobalFlags specifies a value that can add more global flags to the
	// supercommand which will also be available on all subcommands.
	GlobalFlags     FlagAdder
	MissingCallback MissingCallback
	Aliases         []string
	Version         string
	// VersionDetail is a freeform information that is output when the default version
	// subcommand is passed --all. Output is formatted using the user-selected formatter.
	// Exported fields should specify yaml and json field tags.
	VersionDetail interface{}

	// UserAliasesFilename refers to the location of a file that contains
	//   name = cmd [args...]
	// values, that is used to change default behaviour of commands in order
	// to add flags, or provide short cuts to longer commands.
	UserAliasesFilename string

	// FlagKnownAs allows different projects to customise what their flags are
	// known as, e.g. 'flag', 'option', 'item'. All error/log messages
	// will use that name when referring to an individual items/flags in this command.
	// For example, if this value is 'option', the default message 'value for flag'
	// will become 'value for option'.
	FlagKnownAs string
}

// FlagAdder represents a value that has associated flags.
type FlagAdder interface {
	// AddsFlags adds the value's flags to the given flag set.
	AddFlags(*gnuflag.FlagSet)
}

// NewSuperCommand creates and initializes a new `SuperCommand`, and returns
// the fully initialized structure.
func NewSuperCommand(params SuperCommandParams) *SuperCommand {
	command := &SuperCommand{
		Name:    params.Name,
		Purpose: params.Purpose,
		Doc:     params.Doc,
		Log:     params.Log,
		Aliases: params.Aliases,

		globalFlags:         params.GlobalFlags,
		usagePrefix:         params.UsagePrefix,
		missingCallback:     params.MissingCallback,
		version:             params.Version,
		versionDetail:       params.VersionDetail,
		notifyRun:           params.NotifyRun,
		notifyHelp:          params.NotifyHelp,
		userAliasesFilename: params.UserAliasesFilename,
		FlagKnownAs:         params.FlagKnownAs,
	}
	command.init()
	return command
}

// DeprecationCheck is used to provide callbacks to determine if
// a command is deprecated or obsolete.
type DeprecationCheck interface {
	// Deprecated aliases emit a warning when executed. If the command is
	// deprecated, the second return value recommends what to use instead.
	Deprecated() (bool, string)

	// Obsolete aliases are not actually registered. The purpose of this
	// is to allow code to indicate ahead of time some way to determine
	// that the command should stop working.
	Obsolete() bool
}

type commandReference struct {
	name    string
	command Command
	alias   string
	check   DeprecationCheck
}

// SuperCommand is a Command that selects a subcommand and assumes its
// properties; any command line arguments that were not used in selecting
// the subcommand are passed down to it, and to Run a SuperCommand is to run
// its selected subcommand.
type SuperCommand struct {
	CommandBase
	Name                string
	Purpose             string
	Doc                 string
	Log                 *Log
	Aliases             []string
	globalFlags         FlagAdder
	version             string
	versionDetail       interface{}
	usagePrefix         string
	userAliasesFilename string
	userAliases         map[string][]string
	subcmds             map[string]commandReference
	help                *helpCommand
	commonflags         *gnuflag.FlagSet
	flags               *gnuflag.FlagSet
	action              commandReference
	showHelp            bool
	showDescription     bool
	showVersion         bool
	noAlias             bool
	missingCallback     MissingCallback
	notifyRun           func(string)
	notifyHelp          func([]string)

	// FlagKnownAs allows different projects to customise what their flags are
	// known as, e.g. 'flag', 'option', 'item'. All error/log messages
	// will use that name when referring to an individual items/flags in this command.
	// For example, if this value is 'option', the default message 'value for flag'
	// will become 'value for option'.
	FlagKnownAs string
}

// IsSuperCommand implements Command.IsSuperCommand
func (c *SuperCommand) IsSuperCommand() bool {
	return true
}

func (c *SuperCommand) init() {
	if c.subcmds != nil {
		return
	}
	if c.FlagKnownAs == "" {
		// For backward compatibility, the default is 'flag'.
		c.FlagKnownAs = "flag"
	}
	c.help = &helpCommand{
		super: c,
	}
	c.help.init()
	c.subcmds = map[string]commandReference{
		"help": {command: c.help},
	}
	if c.version != "" {
		c.subcmds["version"] = commandReference{
			command: newVersionCommand(c.version, c.versionDetail),
		}
	}

	c.userAliases = ParseAliasFile(c.userAliasesFilename)
}

// AddHelpTopic adds a new help topic with the description being the short
// param, and the full text being the long param.  The description is shown in
// 'help topics', and the full text is shown when the command 'help <name>' is
// called.
func (c *SuperCommand) AddHelpTopic(name, short, long string, aliases ...string) {
	c.help.addTopic(name, short, echo(long), aliases...)
}

// AddHelpTopicCallback adds a new help topic with the description being the
// short param, and the full text being defined by the callback function.
func (c *SuperCommand) AddHelpTopicCallback(name, short string, longCallback func() string) {
	c.help.addTopic(name, short, longCallback)
}

// Register makes a subcommand available for use on the command line. The
// command will be available via its own name, and via any supplied aliases.
func (c *SuperCommand) Register(subcmd Command) {
	info := subcmd.Info()
	c.insert(commandReference{name: info.Name, command: subcmd})
	for _, name := range info.Aliases {
		c.insert(commandReference{name: name, command: subcmd, alias: info.Name})
	}
}

// RegisterDeprecated makes a subcommand available for use on the command line if it
// is not obsolete.  It inserts the command with the specified DeprecationCheck so
// that a warning is displayed if the command is deprecated.
func (c *SuperCommand) RegisterDeprecated(subcmd Command, check DeprecationCheck) {
	if subcmd == nil {
		return
	}

	info := subcmd.Info()
	if check != nil && check.Obsolete() {
		logger.Infof("%q command not registered as it is obsolete", info.Name)
		return
	}
	c.insert(commandReference{name: info.Name, command: subcmd, check: check})
	for _, name := range info.Aliases {
		c.insert(commandReference{name: name, command: subcmd, alias: info.Name, check: check})
	}
}

// RegisterAlias makes an existing subcommand available under another name.
// If `check` is supplied, and the result of the `Obsolete` call is true,
// then the alias is not registered.
func (c *SuperCommand) RegisterAlias(name, forName string, check DeprecationCheck) {
	if check != nil && check.Obsolete() {
		logger.Infof("%q alias not registered as it is obsolete", name)
		return
	}
	action, found := c.subcmds[forName]
	if !found {
		panic(fmt.Sprintf("%q not found when registering alias", forName))
	}
	c.insert(commandReference{
		name:    name,
		command: action.command,
		alias:   forName,
		check:   check,
	})
}

// RegisterSuperAlias makes a subcommand of a registered supercommand
// available under another name. This is useful when the command structure is
// being refactored.  If `check` is supplied, and the result of the `Obsolete`
// call is true, then the alias is not registered.
func (c *SuperCommand) RegisterSuperAlias(name, super, forName string, check DeprecationCheck) {
	if check != nil && check.Obsolete() {
		logger.Infof("%q alias not registered as it is obsolete", name)
		return
	}
	action, found := c.subcmds[super]
	if !found {
		panic(fmt.Sprintf("%q not found when registering alias", super))
	}
	if !action.command.IsSuperCommand() {
		panic(fmt.Sprintf("%q is not a SuperCommand", super))
	}
	superCmd := action.command.(*SuperCommand)

	action, found = superCmd.subcmds[forName]
	if !found {
		panic(fmt.Sprintf("%q not found as a command in %q", forName, super))
	}

	c.insert(commandReference{
		name:    name,
		command: action.command,
		alias:   super + " " + forName,
		check:   check,
	})
}

func (c *SuperCommand) insert(value commandReference) {
	if _, found := c.subcmds[value.name]; found {
		panic(fmt.Sprintf("command already registered: %q", value.name))
	}
	c.subcmds[value.name] = value
}

// describeCommands returns a short description of each registered subcommand.
func (c *SuperCommand) describeCommands(simple bool) string {
	var lineFormat = "    %-*s - %s"
	var outputFormat = "commands:\n%s"
	if simple {
		lineFormat = "%-*s  %s"
		outputFormat = "%s"
	}
	cmds := make([]string, len(c.subcmds))
	i := 0
	longest := 0
	for name := range c.subcmds {
		if len(name) > longest {
			longest = len(name)
		}
		cmds[i] = name
		i++
	}
	sort.Strings(cmds)
	var result []string
	for _, name := range cmds {
		action := c.subcmds[name]
		if deprecated, _ := action.Deprecated(); deprecated {
			continue
		}
		info := action.command.Info()
		purpose := info.Purpose
		if action.alias != "" {
			purpose = "Alias for '" + action.alias + "'."
		}
		result = append(result, fmt.Sprintf(lineFormat, longest, name, purpose))
	}
	return fmt.Sprintf(outputFormat, strings.Join(result, "\n"))
}

// Info returns a description of the currently selected subcommand, or of the
// SuperCommand itself if no subcommand has been specified.
func (c *SuperCommand) Info() *Info {
	if c.action.command != nil {
		info := *c.action.command.Info()
		info.Name = fmt.Sprintf("%s %s", c.Name, info.Name)
		info.FlagKnownAs = c.FlagKnownAs
		return &info
	}
	docParts := []string{}
	if doc := strings.TrimSpace(c.Doc); doc != "" {
		docParts = append(docParts, doc)
	}
	if cmds := c.describeCommands(false); cmds != "" {
		docParts = append(docParts, cmds)
	}
	return &Info{
		Name:        c.Name,
		Args:        "<command> ...",
		Purpose:     c.Purpose,
		Doc:         strings.Join(docParts, "\n\n"),
		Aliases:     c.Aliases,
		FlagKnownAs: c.FlagKnownAs,
	}
}

const helpPurpose = "Show help on a command or other topic."

// SetCommonFlags creates a new "commonflags" flagset, whose
// flags are shared with the argument f; this enables us to
// add non-global flags to f, which do not carry into subcommands.
func (c *SuperCommand) SetCommonFlags(f *gnuflag.FlagSet) {
	if c.Log != nil {
		c.Log.AddFlags(f)
	}
	if c.globalFlags != nil {
		c.globalFlags.AddFlags(f)
	}
	f.BoolVar(&c.showHelp, "h", false, helpPurpose)
	f.BoolVar(&c.showHelp, "help", false, "")
	// In the case where we are providing the basis for a plugin,
	// plugins are required to support the --description argument.
	// The Purpose attribute will be printed (if defined), allowing
	// plugins to provide a sensible line of text for 'juju help plugins'.
	f.BoolVar(&c.showDescription, "description", false, "Show short description of plugin, if any")
	c.commonflags = gnuflag.NewFlagSetWithFlagKnownAs(c.Info().Name, gnuflag.ContinueOnError, FlagAlias(c, "flag"))
	c.commonflags.SetOutput(ioutil.Discard)
	f.VisitAll(func(flag *gnuflag.Flag) {
		c.commonflags.Var(flag.Value, flag.Name, flag.Usage)
	})
}

// SetFlags adds the options that apply to all commands, particularly those
// due to logging.
func (c *SuperCommand) SetFlags(f *gnuflag.FlagSet) {
	c.SetCommonFlags(f)
	// Only flags set by SetCommonFlags are passed on to subcommands.
	// Any flags added below only take effect when no subcommand is
	// specified (e.g. command --version).
	if c.version != "" {
		f.BoolVar(&c.showVersion, "version", false, "show the command's version and exit")
	}
	if c.userAliasesFilename != "" {
		f.BoolVar(&c.noAlias, "no-alias", false, "do not process command aliases when running this command")
	}
	c.flags = f
}

// For a SuperCommand, we want to parse the args with
// allowIntersperse=false. This will mean that the args may contain other
// options that haven't been defined yet, and that only options that relate
// to the SuperCommand itself can come prior to the subcommand name.
func (c *SuperCommand) AllowInterspersedFlags() bool {
	return false
}

// Init initializes the command for running.
func (c *SuperCommand) Init(args []string) error {
	if c.showDescription {
		return CheckEmpty(args)
	}
	if len(args) == 0 {
		c.action = c.subcmds["help"]
		return c.action.command.Init(args)
	}

	if userAlias, found := c.userAliases[args[0]]; found && !c.noAlias {
		logger.Debugf("using alias %q=%q", args[0], strings.Join(userAlias, " "))
		args = append(userAlias, args[1:]...)
	}
	found := false
	// Look for the command.
	if c.action, found = c.subcmds[args[0]]; !found {
		if c.missingCallback != nil {
			c.action = commandReference{
				command: &missingCommand{
					callback:  c.missingCallback,
					superName: c.Name,
					name:      args[0],
					args:      args[1:],
				},
			}
			// Yes return here, no Init called on missing Command.
			return nil
		}
		return fmt.Errorf("unrecognized command: %s %s", c.Name, args[0])
	}
	args = args[1:]
	subcmd := c.action.command
	if subcmd.IsSuperCommand() {
		f := gnuflag.NewFlagSetWithFlagKnownAs(c.Info().Name, gnuflag.ContinueOnError, FlagAlias(subcmd, "flag"))
		f.SetOutput(ioutil.Discard)
		subcmd.SetFlags(f)
	} else {
		subcmd.SetFlags(c.commonflags)
	}
	if err := c.commonflags.Parse(subcmd.AllowInterspersedFlags(), args); err != nil {
		return err
	}
	args = c.commonflags.Args()
	if c.showHelp {
		// We want to treat help for the command the same way we would if we went "help foo".
		args = []string{c.action.name}
		c.action = c.subcmds["help"]
	}
	return c.action.command.Init(args)
}

// Run executes the subcommand that was selected in Init.
func (c *SuperCommand) Run(ctx *Context) error {
	if c.showDescription {
		if c.Purpose != "" {
			fmt.Fprintf(ctx.Stdout, "%s\n", c.Purpose)
		} else {
			fmt.Fprintf(ctx.Stdout, "%s: no description available\n", c.Info().Name)
		}
		return nil
	}
	if c.action.command == nil {
		panic("Run: missing subcommand; Init failed or not called")
	}
	if c.Log != nil {
		if err := c.Log.Start(ctx); err != nil {
			return err
		}
	}
	if c.notifyRun != nil {
		name := c.Name
		if c.usagePrefix != "" && c.usagePrefix != name {
			name = c.usagePrefix + " " + name
		}
		c.notifyRun(name)
	}
	if deprecated, replacement := c.action.Deprecated(); deprecated {
		ctx.Infof("WARNING: %q is deprecated, please use %q", c.action.name, replacement)
	}
	err := c.action.command.Run(ctx)
	if err != nil && !IsErrSilent(err) {
		if IsErrSilentPrintError(err) {
			Write(ctx.Stderr, err)
		} else {
			WriteError(ctx.Stderr, err)
		}
		logger.Debugf("error stack: \n%v", errors.ErrorStack(err))
		// Now that this has been logged, don't log again in cmd.Main.
		if !IsRcPassthroughError(err) {
			err = ErrSilent
		}
	} else {
		logger.Infof("command finished")
	}
	return err
}

// FindClosestSubCommand attempts to find a sub command by a given name.
// This is used to help locate potential commands where the name isn't an
// exact match.
// If the resulting fuzzy match algorithm returns a value that is itself too
// far away from the size of the word, we disgard that and say a match isn't
// relavent i.e. "foo" "barsomethingfoo" would not match
func (c *SuperCommand) FindClosestSubCommand(name string) (string, Command, bool) {
	// Exit early if there are no subcmds
	if len(c.subcmds) == 0 {
		return "", nil, false
	}

	// Attempt to find the closest match of a substring.
	type Indexed = struct {
		Name  string
		Value int
	}
	matches := make([]Indexed, 0, len(c.subcmds))
	for cmdName := range c.subcmds {
		matches = append(matches, Indexed{
			Name:  cmdName,
			Value: levenshteinDistance(name, cmdName),
		})
	}
	// Find the smallest levenshtein distance. If two values are the same,
	// fallback to sorting on the name, which should give predictable results.
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Value < matches[j].Value {
			return true
		}
		if matches[i].Value > matches[j].Value {
			return false
		}
		return matches[i].Name < matches[j].Name
	})
	matchedName := matches[0].Name
	matchedValue := matches[0].Value

	// If the matched value is less than the length+1 of the string, fail the
	// match.
	if _, ok := c.subcmds[matchedName]; ok && matchedName != "" && matchedValue < len(matchedName)+1 {
		return matchedName, c.subcmds[matchedName].command, true
	}
	return "", nil, false
}

// levenshteinDistance
// from https://groups.google.com/forum/#!topic/golang-nuts/YyH1f_qCZVc
// (no min, compute lengths once, 2 rows array)
// fastest profiled
func levenshteinDistance(a, b string) int {
	la := len(a)
	lb := len(b)
	d := make([]int, la+1)
	var lastdiag, olddiag, temp int

	for i := 1; i <= la; i++ {
		d[i] = i
	}
	for i := 1; i <= lb; i++ {
		d[0] = i
		lastdiag = i - 1
		for j := 1; j <= la; j++ {
			olddiag = d[j]
			min := d[j] + 1
			if (d[j-1] + 1) < min {
				min = d[j-1] + 1
			}
			if a[j-1] == b[i-1] {
				temp = 0
			} else {
				temp = 1
			}
			if (lastdiag + temp) < min {
				min = lastdiag + temp
			}
			d[j] = min
			lastdiag = olddiag
		}
	}
	return d[la]
}

type missingCommand struct {
	CommandBase
	callback  MissingCallback
	superName string
	name      string
	args      []string
}

// Missing commands only need to supply Info for the interface, but this is
// never called.
func (c *missingCommand) Info() *Info {
	return nil
}

func (c *missingCommand) Run(ctx *Context) error {
	err := c.callback(ctx, c.name, c.args)
	_, isUnrecognized := err.(*UnrecognizedCommand)
	if !isUnrecognized {
		return err
	}
	return DefaultUnrecognizedCommand(fmt.Sprintf("%s %s", c.superName, c.name))
}

// Deprecated calls into the check interface if one was specified,
// otherwise it says the command isn't deprecated.
func (r commandReference) Deprecated() (bool, string) {
	if r.check == nil {
		return false, ""
	}
	return r.check.Deprecated()
}
