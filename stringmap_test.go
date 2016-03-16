// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package cmd

import (
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
)

var _ = gc.Suite(&StringMapSuite{})

type StringMapSuite struct {
	testing.IsolationSuite
}

func (StringMapSuite) TestStringMapNilOk(c *gc.C) {
	// note that the map may start out nil
	var values map[string]string
	c.Assert(values, gc.IsNil)
	sm := StringMap{&values}
	err := sm.Set("foo=foovalue")
	c.Assert(err, jc.ErrorIsNil)
	err = sm.Set("bar=barvalue")
	c.Assert(err, jc.ErrorIsNil)

	// now the map is non-nil and filled
	c.Assert(values, gc.DeepEquals, map[string]string{
		"foo": "foovalue",
		"bar": "barvalue",
	})
}

func (StringMapSuite) TestStringMapBadVal(c *gc.C) {
	sm := StringMap{&map[string]string{}}
	err := sm.Set("foo")
	c.Assert(err, gc.ErrorMatches, "badly formatted name value pair: foo")
}

func (StringMapSuite) TestStringMapDupVal(c *gc.C) {
	sm := StringMap{&map[string]string{}}
	err := sm.Set("bar=somevalue")
	c.Assert(err, jc.ErrorIsNil)
	err = sm.Set("bar=someothervalue")
	c.Assert(err, gc.ErrorMatches, ".*duplicate.*bar.*")
}
