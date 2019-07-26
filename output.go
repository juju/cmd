// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/juju/errors"
	"github.com/juju/gnuflag"
	goyaml "gopkg.in/yaml.v2"
)

// Formatter writes the arbitrary object into the writer.
type Formatter interface {
	Format(writer io.Writer, value interface{}) error
}

// FormatterFunc writes the arbitrary object into the writer.
type FormatterFunc func(writer io.Writer, value interface{}) error

// Format writes the arbitrary object into the writer.
func (f FormatterFunc) Format(writer io.Writer, value interface{}) error {
	return f(writer, value)
}

// FormatterWithArgument writes the arbitrary object into the writer using
// an optional argument.
type FormatterWithArgument interface {
	Formatter
	// FormatWithArg is called instead when the formatter is passed an argument.
	FormatWithArg(writer io.Writer, arg string, value interface{}) error
	// ValidateArg is called when the formatter is selected with an argument.
	// An error should be returned when the argument is malformed.
	ValidateArg(arg string) error
}

// formatYamlFunc writes out value as yaml to the writer, unless value is nil.
func formatYamlFunc(writer io.Writer, value interface{}) error {
	if value == nil {
		return nil
	}
	result, err := goyaml.Marshal(value)
	if err != nil {
		return err
	}
	for i := len(result) - 1; i > 0; i-- {
		if result[i] != '\n' {
			break
		}
		result = result[:i]
	}

	if len(result) > 0 {
		result = append(result, '\n')
		_, err = writer.Write(result)
		return err
	}
	return nil
}

// formatJsonFunc writes out value as json.
func formatJsonFunc(writer io.Writer, value interface{}) error {
	result, err := json.Marshal(value)
	if err != nil {
		return err
	}
	result = append(result, '\n')
	_, err = writer.Write(result)
	return err
}

// formatSmartFunc marshals value into a []byte according to the following rules:
//   * string:        untouched
//   * bool:          converted to `True` or `False` (to match pyjuju)
//   * int or float:  converted to sensible strings
//   * []string:      joined by `\n`s into a single string
//   * anything else: delegate to FormatYaml
func formatSmartFunc(writer io.Writer, value interface{}) error {
	if value == nil {
		return nil
	}
	valueStr := ""
	switch value := value.(type) {
	case string:
		valueStr = value
	case []string:
		valueStr = strings.Join(value, "\n")
	case bool:
		if value {
			valueStr = "True"
		} else {
			valueStr = "False"
		}
	default:
		return FormatYaml(writer, value)
	}
	if valueStr == "" {
		return nil
	}
	_, err := writer.Write([]byte(valueStr + "\n"))
	return err
}

// formatTemplate writes out value according to the gotemplate arg.
type formatTemplate struct{}

func (f formatTemplate) Format(writer io.Writer, value interface{}) error {
	return errors.New("--format template requires gotemplate argument")
}

func (f formatTemplate) FormatWithArg(writer io.Writer, arg string, value interface{}) error {
	t, err := f.template(arg)
	if err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(t.Execute(writer, value))
}

func (f formatTemplate) ValidateArg(arg string) error {
	_, err := f.template(arg)
	return errors.Trace(err)
}

func (f formatTemplate) template(arg string) (*template.Template, error) {
	arg, err := strconv.Unquote(arg)
	if err != nil {
		return nil, errors.Annotate(err, "template must be passed with quotes")
	}
	t, err := template.New("format-template").Parse(arg)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return t, nil
}

var (
	// FormatSmart marshals value into a []byte according to the following rules:
	//   * string:        untouched
	//   * bool:          converted to `True` or `False` (to match pyjuju)
	//   * int or float:  converted to sensible strings
	//   * []string:      joined by `\n`s into a single string
	//   * anything else: delegate to FormatYaml
	FormatSmart = FormatterFunc(formatSmartFunc)
	// FormatYaml writes out value as yaml to the writer, unless value is nil.
	FormatYaml = FormatterFunc(formatYamlFunc)
	// FormatJson writes out value as json.
	FormatJson = FormatterFunc(formatJsonFunc)
	// FormatTemplate writes out value according to the gotemplate arg.
	FormatTemplate = &formatTemplate{}
)

// DefaultFormatters holds the formatters that can be
// specified with the --format flag.
var DefaultFormatters = map[string]Formatter{
	"smart":    FormatSmart,
	"yaml":     FormatYaml,
	"json":     FormatJson,
	"template": FormatTemplate,
}

// formatterValue implements gnuflag.Value for the --format flag.
type formatterValue struct {
	name       string
	arg        string
	formatters map[string]Formatter
}

// newFormatterValue returns a new formatterValue. The initial Formatter name
// must be present in formatters.
func newFormatterValue(initial string, formatters map[string]Formatter) *formatterValue {
	v := &formatterValue{formatters: formatters}
	if err := v.Set(initial); err != nil {
		panic(err)
	}
	return v
}

// Set stores the chosen formatter name in v.name.
func (v *formatterValue) Set(value string) error {
	// formatter<=arg>
	args := strings.SplitN(value, "=", 2)
	name := args[0]
	arg := ""
	if len(args) == 2 {
		arg = args[1]
	}

	formatter := v.formatters[name]
	if formatter == nil {
		return fmt.Errorf("unknown format %q", value)
	}
	v.name = name

	if argFormatter, ok := formatter.(FormatterWithArgument); ok {
		err := argFormatter.ValidateArg(arg)
		if err != nil {
			return errors.Annotatef(err, "formatter %s passed invalid argument %s", name, arg)
		}
		v.arg = arg
	}
	return nil
}

// String returns the chosen formatter name.
func (v *formatterValue) String() string {
	return v.name
}

// doc returns documentation for the --format flag.
func (v *formatterValue) doc() string {
	choices := make([]string, len(v.formatters))
	i := 0
	for name := range v.formatters {
		choices[i] = name
		i++
	}
	sort.Strings(choices)
	return "Specify output format (" + strings.Join(choices, "|") + ")"
}

// Output is responsible for interpreting output-related command line flags
// and writing a value to a file or to stdout as directed.
type Output struct {
	formatter *formatterValue
	outPath   string
}

// AddFlags injects the --format and --output command line flags into f.
func (c *Output) AddFlags(f *gnuflag.FlagSet, defaultFormatter string, formatters map[string]Formatter) {
	c.formatter = newFormatterValue(defaultFormatter, formatters)
	f.Var(c.formatter, "format", c.formatter.doc())
	f.StringVar(&c.outPath, "o", "", "Specify an output file")
	f.StringVar(&c.outPath, "output", "", "")
}

// Write formats and outputs the value as directed by the --format and
// --output command line flags.
func (c *Output) Write(ctx *Context, value interface{}) (err error) {
	formatterName := c.formatter.name
	formatterArg := c.formatter.arg
	formatter := c.formatter.formatters[formatterName]
	// If the formatter is not one of the default ones, add a new line at the end.
	// This keeps consistent behaviour with the current code.
	var newline bool
	if _, found := DefaultFormatters[formatterName]; !found {
		newline = true
	}
	if err := c.writeFormatter(ctx, formatter, formatterArg, value, newline); err != nil {
		return err
	}
	return nil
}

// WriteFormatter formats and outputs the value with the given formatter,
// to the output directed by the --output command line flag.
func (c *Output) WriteFormatter(ctx *Context, formatter Formatter, value interface{}) (err error) {
	return c.writeFormatter(ctx, formatter, "", value, false)
}

func (c *Output) writeFormatter(ctx *Context, formatter Formatter, formatterArg string, value interface{}, newline bool) (err error) {
	var target io.Writer
	if c.outPath == "" {
		target = ctx.Stdout
	} else {
		path := ctx.AbsPath(c.outPath)
		var f *os.File
		if f, err = os.Create(path); err != nil {
			return
		}
		defer f.Close()
		target = f
	}
	if argFormatter, ok := formatter.(FormatterWithArgument); ok && formatterArg != "" {
		if err := argFormatter.FormatWithArg(target, formatterArg, value); err != nil {
			return err
		}
	} else if err := formatter.Format(target, value); err != nil {
		return err
	}
	if newline {
		fmt.Fprintln(target)
	}
	return nil
}

func (c *Output) Name() string {
	return c.formatter.name
}
