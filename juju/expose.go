package main

import (
	"errors"
	"launchpad.net/juju-core/cmd"
	"launchpad.net/juju-core/juju"
)

// ExposeCommand is responsible exposing services.
type ExposeCommand struct {
	EnvCommandBase
	ServiceName string
}

func (c *ExposeCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "expose",
		Args:    "<service>",
		Purpose: "expose a service",
	}
}

func (c *ExposeCommand) Init(args []string) error {
	if len(args) == 0 {
		return errors.New("no service name specified")
	}
	c.ServiceName = args[0]
	return cmd.CheckEmpty(args[1:])
}

// Run changes the juju-managed firewall to expose any
// ports that were also explicitly marked by units as open.
func (c *ExposeCommand) Run(_ *cmd.Context) error {
	conn, err := juju.NewConnFromName(c.EnvName)
	if err != nil {
		return err
	}
	defer conn.Close()
	svc, err := conn.State.Service(c.ServiceName)
	if err != nil {
		return err
	}
	return svc.SetExposed()
}
