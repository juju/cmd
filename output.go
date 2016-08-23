// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/juju/gnuflag"
	goyaml "gopkg.in/yaml.v2"
)

// Formatter writes the arbitrary object into the writer.
type Formatter func(writer io.Writer, value interface{}) error

// FormatYaml writes out value as yaml to the writer, unless value is nil.
func FormatYaml(writer io.Writer, value interface{}) error {
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

// FormatJson writes out value as json.
func FormatJson(writer io.Writer, value interface{}) error {
	result, err := json.Marshal(value)
	if err != nil {
		return err
	}
	result = append(result, '\n')
	_, err = writer.Write(result)
	return err
}

// FormatSmart marshals value into a []byte according to the following rules:
//   * string:        untouched
//   * bool:          converted to `True` or `False` (to match pyjuju)
//   * int or float:  converted to sensible strings
//   * []string:      joined by `\n`s into a single string
//   * anything else: delegate to FormatYaml
func FormatSmart(writer io.Writer, value interface{}) error {
	if value == nil {
		return nil
	}
	v := reflect.ValueOf(value)
	switch kind := v.Kind(); kind {
	case reflect.String:
		if value == "" {
			return nil
		}
		_, err := fmt.Fprintln(writer, value)
		return err
	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.String {
			slice := reflect.MakeSlice(reflect.TypeOf([]string(nil)), v.Len(), v.Len())
			reflect.Copy(slice, v)
			_, err := fmt.Fprintln(writer, strings.Join(slice.Interface().([]string), "\n"))
			return err
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.String {
			_, err := fmt.Fprintln(writer, strings.Join(value.([]string), "\n"))
			return err
		}
	case reflect.Bool:
		result := "False"
		if value.(bool) {
			result = "True"
		}
		_, err := fmt.Fprintln(writer, result)
		return err
	case reflect.Float32, reflect.Float64:
		sv := strconv.FormatFloat(value.(float64), 'f', -1, 64)
		_, err := fmt.Fprintln(writer, sv)
		return err
	case reflect.Map:
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return fmt.Errorf("cannot marshal %#v", value)
	}
	return FormatYaml(writer, value)
}

// DefaultFormatters holds the formatters that can be
// specified with the --format flag.
var DefaultFormatters = map[string]Formatter{
	"smart": FormatSmart,
	"yaml":  FormatYaml,
	"json":  FormatJson,
}

// formatterValue implements gnuflag.Value for the --format flag.
type formatterValue struct {
	name       string
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
	if v.formatters[value] == nil {
		return fmt.Errorf("unknown format %q", value)
	}
	v.name = value
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

// format runs the chosen formatter on value.
func (v *formatterValue) format(writer io.Writer, value interface{}) error {
	return v.formatters[v.name](writer, value)
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

	if err = c.formatter.format(target, value); err != nil {
		return
	}
	// If the formatter is not one of the default ones, add a new line at the end.
	// This keeps consistent behaviour with the current code.
	if _, found := DefaultFormatters[c.formatter.name]; !found {
		fmt.Fprintln(target)
	}
	return
}

func (c *Output) Name() string {
	return c.formatter.name
}
