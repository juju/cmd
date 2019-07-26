# Copyright 2014 Canonical Ltd.
# Licensed under the LGPLv3, see LICENSE file for details.

default: check

check:
	go test

docs:
	godoc2md gopkg.in/juju/cmd.v2 > README.md

