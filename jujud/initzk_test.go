package main_test

import (
	. "launchpad.net/gocheck"
	"launchpad.net/juju/go/cmd"
	main "launchpad.net/juju/go/cmd/jujud"
)

type InitzkSuite struct{}

var _ = Suite(&InitzkSuite{})

func parseInitzkCommand(args []string) (*main.InitzkCommand, error) {
	c := &main.InitzkCommand{}
	err := cmd.Parse(c, true, args)
	return c, err
}

func (s *InitzkSuite) TestParse(c *C) {
	args := []string{}
	_, err := parseInitzkCommand(args)
	c.Assert(err, ErrorMatches, "--instance-id option must be set")

	args = append(args, "--instance-id", "iWhatever")
	_, err = parseInitzkCommand(args)
	c.Assert(err, ErrorMatches, "--provider-type option must be set")

	args = append(args, "--provider-type", "dummy")
	izk, err := parseInitzkCommand(args)
	c.Assert(err, IsNil)
	c.Assert(izk.Zookeeper, Equals, "127.0.0.1:2181")
	c.Assert(izk.InstanceId, Equals, "iWhatever")
	c.Assert(izk.ProviderType, Equals, "dummy")

	args = append(args, "--zookeeper-servers", "zk")
	izk, err = parseInitzkCommand(args)
	c.Assert(err, IsNil)
	c.Assert(izk.Zookeeper, Equals, "zk")

	args = append(args, "haha disregard that")
	_, err = parseInitzkCommand(args)
	c.Assert(err, ErrorMatches, `unrecognised args: \[haha disregard that\]`)
}