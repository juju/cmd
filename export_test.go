// Copyright 2017 Canonical Ltd.
// Licensed under the LGPLv3, see LICENSE file for details.

package cmd

func NewVersionCommand(version string, versionDetail interface{}) Command {
	return newVersionCommand(version, versionDetail)
}
