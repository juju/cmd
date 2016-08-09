// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/juju/loggo"
	"github.com/juju/gnuflag"
)

// Log supplies the necessary functionality for Commands that wish to set up
// logging.
type Log struct {
	// If DefaultConfig is set, it will be used for the
	// default logging configuration.
	DefaultConfig string
	Path          string
	Verbose       bool
	Quiet         bool
	Debug         bool
	ShowLog       bool
	Config        string

	// NewWriter creates a new logging writer for a specified target.
	NewWriter func(target io.Writer) loggo.Writer
}

// GetLogWriter returns a logging writer for the specified target.
func (l *Log) GetLogWriter(target io.Writer) loggo.Writer {
	if l.NewWriter != nil {
		return l.NewWriter(target)
	}
	return loggo.NewSimpleWriter(target, loggo.DefaultFormatter)
}

// AddFlags adds appropriate flags to f.
func (l *Log) AddFlags(f *gnuflag.FlagSet) {
	f.StringVar(&l.Path, "log-file", "", "path to write log to")
	f.BoolVar(&l.Verbose, "v", false, "show more verbose output")
	f.BoolVar(&l.Verbose, "verbose", false, "show more verbose output")
	f.BoolVar(&l.Quiet, "q", false, "show no informational output")
	f.BoolVar(&l.Quiet, "quiet", false, "show no informational output")
	f.BoolVar(&l.Debug, "debug", false, "equivalent to --show-log --log-config=<root>=DEBUG")
	f.StringVar(&l.Config, "logging-config", l.DefaultConfig, "specify log levels for modules")
	f.BoolVar(&l.ShowLog, "show-log", false, "if set, write the log file to stderr")
}

// Start starts logging using the given Context.
func (log *Log) Start(ctx *Context) error {
	if log.Verbose && log.Quiet {
		return fmt.Errorf(`"verbose" and "quiet" flags clash, please use one or the other, not both`)
	}
	ctx.quiet = log.Quiet
	ctx.verbose = log.Verbose
	if log.Path != "" {
		path := ctx.AbsPath(log.Path)
		target, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		writer := log.GetLogWriter(target)
		err = loggo.RegisterWriter("logfile", writer)
		if err != nil {
			return err
		}
	}
	level := loggo.WARNING
	if log.ShowLog {
		level = loggo.INFO
	}
	if log.Debug {
		log.ShowLog = true
		level = loggo.DEBUG
		// override quiet or verbose if set, this way all the information goes
		// to the log file.
		ctx.quiet = true
		ctx.verbose = false
	}

	if log.ShowLog {
		// We replace the default writer to use ctx.Stderr rather than os.Stderr.
		writer := log.GetLogWriter(ctx.Stderr)
		_, err := loggo.ReplaceDefaultWriter(writer)
		if err != nil {
			return err
		}
	} else {
		loggo.RemoveWriter("default")
		// Create a simple writer that doesn't show filenames, or timestamps,
		// and only shows warning or above.
		writer := loggo.NewSimpleWriter(ctx.Stderr, warningFormatter)
		writer = loggo.NewMinimumLevelWriter(writer, loggo.WARNING)
		err := loggo.RegisterWriter("warning", writer)
		if err != nil {
			return err
		}
	}
	// Set the level on the root logger.
	root := loggo.GetLogger("")
	root.SetLogLevel(level)
	// Override the logging config with specified logging config.
	loggo.ConfigureLoggers(log.Config)
	return nil
}

// warningFormatter is a simple loggo formatter that produces something like:
//   WARNING The message...
func warningFormatter(entry loggo.Entry) string {
	return fmt.Sprintf("%s %s", entry.Level, entry.Message)
}

// NewCommandLogWriter creates a loggo writer for registration
// by the callers of a command. This way the logged output can also
// be displayed otherwise, e.g. on the screen.
func NewCommandLogWriter(name string, out, err io.Writer) loggo.Writer {
	return &commandLogWriter{name, out, err}
}

// commandLogWriter filters the log messages for name.
type commandLogWriter struct {
	name string
	out  io.Writer
	err  io.Writer
}

// Write implements loggo's Writer interface.
func (s *commandLogWriter) Write(entry loggo.Entry) {
	if entry.Module == s.name {
		if entry.Level <= loggo.INFO {
			fmt.Fprintf(s.out, "%s\n", entry.Message)
		} else {
			fmt.Fprintf(s.err, "%s\n", entry.Message)
		}
	}
}
