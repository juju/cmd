// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

import (
	"fmt"
	"strings"
)

// StringMap is a type that deserializes a CLI string using gnuflag's Value
// semantics.  It expects a name=value pair, and supports multiple copies of the
// flag adding more pairs, though the names must be unique.
type StringMap struct {
	Mapping *map[string]string
}

// Set implements gnuflag.Value's Set method.
func (m StringMap) Set(s string) error {
	if *m.Mapping == nil {
		*m.Mapping = map[string]string{}
	}
	// make a copy so the following code is less ugly with dereferencing.
	mapping := *m.Mapping

	vals := strings.SplitN(s, "=", 2)
	if len(vals) != 2 {
		return fmt.Errorf("badly formatted name value pair: " + s)
	}
	name, value := vals[0], vals[1]
	if _, ok := mapping[name]; ok {
		return fmt.Errorf("duplicate name specified: %q", name)
	}
	mapping[name] = value
	return nil
}

// String implements gnuflag.Value's String method
func (m StringMap) String() string {
	pairs := make([]string, 0, len(*m.Mapping))
	for name, value := range *m.Mapping {
		pairs = append(pairs, name+"="+value)
	}
	return strings.Join(pairs, ";")
}
