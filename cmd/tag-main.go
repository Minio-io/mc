/*
 * MinIO Client (C) 2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"github.com/minio/cli"
)

var tagCmd = cli.Command{
	Name:            "tag",
	Usage:           "manage tags for an object",
	Action:          mainTag,
	Before:          setGlobalsFromContext,
	Flags:           globalFlags,
	HideHelpCommand: true,
	Subcommands: []cli.Command{
		tagListCmd,
		tagRemoveCmd,
		tagSetCmd,
	},
}

func checkMainTagSyntax(ctx *cli.Context) {
	cli.ShowCommandHelp(ctx, "")
}

func mainTag(ctx *cli.Context) error {
	checkMainTagSyntax(ctx)
	return nil
}
