// Copyright (c) 2015-2022 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/minio/cli"
	json "github.com/minio/colorjson"
	"github.com/minio/mc/pkg/probe"
	"github.com/minio/pkg/v3/console"
)

// diff specific flags.
var (
	diffFlags = []cli.Flag{}
)

// Compute differences in object name, size, and date between two buckets.
var diffCmd = cli.Command{
	Name:         "diff",
	Usage:        "list differences in object name, size, and date between two buckets",
	Action:       mainDiff,
	OnUsageError: onUsageError,
	Before:       setGlobalsFromContext,
	Flags:        append(diffFlags, globalFlags...),
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} [FLAGS] SOURCE TARGET

FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}
DESCRIPTION:
  Diff only calculates differences in object name, size and time. It *DOES NOT* compare objects' contents.

LEGEND:
  < - object is only in source.
  > - object is only in destination.
  ! - newer object is in source.

EXAMPLES:
  1. Compare a local folder with a folder on Amazon S3 cloud storage.
     {{.Prompt}} {{.HelpName}} ~/Photos s3/mybucket/Photos

  2. Compare two folders on a local filesystem.
     {{.Prompt}} {{.HelpName}} ~/Photos /Media/Backup/Photos
`,
}

// diffMessage json container for diff messages
type diffMessage struct {
	Status        string       `json:"status"`
	FirstURL      string       `json:"first"`
	SecondURL     string       `json:"second"`
	Diff          differType   `json:"diff"`
	Error         *probe.Error `json:"error,omitempty"`
	firstContent  *ClientContent
	secondContent *ClientContent
}

// String colorized diff message
func (d diffMessage) String() string {
	msg := ""
	switch d.Diff {
	case differInFirst:
		msg = console.Colorize("DiffOnlyInFirst", "< "+d.FirstURL)
	case differInSecond:
		msg = console.Colorize("DiffOnlyInSecond", "> "+d.SecondURL)
	case differInType:
		msg = console.Colorize("DiffType", "! "+d.SecondURL)
	case differInSize:
		msg = console.Colorize("DiffSize", "! "+d.SecondURL)
	case differInMetadata:
		msg = console.Colorize("DiffMetadata", "! "+d.SecondURL)
	case differInAASourceMTime:
		msg = console.Colorize("DiffMMSourceMTime", "! "+d.SecondURL)
	case differInNone:
		msg = console.Colorize("DiffInNone", "= "+d.FirstURL)
	default:
		fatalIf(errDummy().Trace(d.FirstURL, d.SecondURL),
			"Unhandled difference between `"+d.FirstURL+"` and `"+d.SecondURL+"`.")
	}
	return msg
}

// JSON jsonified diff message
func (d diffMessage) JSON() string {
	d.Status = "success"
	diffJSONBytes, e := json.MarshalIndent(d, "", " ")
	fatalIf(probe.NewError(e),
		"Unable to marshal diff message `"+d.FirstURL+"`, `"+d.SecondURL+"` and `"+fmt.Sprint(d.Diff)+"`.")
	return string(diffJSONBytes)
}

func checkDiffSyntax(ctx context.Context, cliCtx *cli.Context, encKeyDB map[string][]prefixSSEPair) {
	if len(cliCtx.Args()) != 2 {
		showCommandHelpAndExit(cliCtx, 1) // last argument is exit code
	}
	for _, arg := range cliCtx.Args() {
		if strings.TrimSpace(arg) == "" {
			fatalIf(errInvalidArgument().Trace(cliCtx.Args()...), "Unable to validate empty argument.")
		}
	}
	URLs := cliCtx.Args()
	firstURL := URLs[0]
	secondURL := URLs[1]

	// Diff only works between two directories, verify them below.

	// Verify if firstURL is accessible.
	_, firstContent, err := url2Stat(ctx, url2StatOptions{urlStr: firstURL, versionID: "", fileAttr: false, encKeyDB: encKeyDB, timeRef: time.Time{}, isZip: false, ignoreBucketExistsCheck: false})
	if err != nil {
		fatalIf(err.Trace(firstURL), fmt.Sprintf("Unable to stat '%s'.", firstURL))
	}

	// Verify if its a directory.
	if !firstContent.Type.IsDir() {
		fatalIf(errInvalidArgument().Trace(firstURL), fmt.Sprintf("`%s` is not a folder.", firstURL))
	}

	// Verify if secondURL is accessible.
	_, secondContent, err := url2Stat(ctx, url2StatOptions{urlStr: secondURL, versionID: "", fileAttr: false, encKeyDB: encKeyDB, timeRef: time.Time{}, isZip: false, ignoreBucketExistsCheck: false})
	if err != nil {
		// Destination doesn't exist is okay.
		if _, ok := err.ToGoError().(ObjectMissing); !ok {
			fatalIf(err.Trace(secondURL), fmt.Sprintf("Unable to stat '%s'.", secondURL))
		}
	}

	// Verify if its a directory.
	if err == nil && !secondContent.Type.IsDir() {
		fatalIf(errInvalidArgument().Trace(secondURL), fmt.Sprintf("`%s` is not a folder.", secondURL))
	}
}

// doDiffMain runs the diff.
func doDiffMain(ctx context.Context, firstURL, secondURL string) error {
	// Source and targets are always directories
	sourceSeparator := string(newClientURL(firstURL).Separator)
	if !strings.HasSuffix(firstURL, sourceSeparator) {
		firstURL = firstURL + sourceSeparator
	}
	targetSeparator := string(newClientURL(secondURL).Separator)
	if !strings.HasSuffix(secondURL, targetSeparator) {
		secondURL = secondURL + targetSeparator
	}

	// Expand aliased urls.
	firstAlias, firstURL, _ := mustExpandAlias(firstURL)
	secondAlias, secondURL, _ := mustExpandAlias(secondURL)

	firstClient, err := newClientFromAlias(firstAlias, firstURL)
	if err != nil {
		fatalIf(err.Trace(firstAlias, firstURL, secondAlias, secondURL),
			fmt.Sprintf("Failed to diff '%s' and '%s'", firstURL, secondURL))
	}

	secondClient, err := newClientFromAlias(secondAlias, secondURL)
	if err != nil {
		fatalIf(err.Trace(firstAlias, firstURL, secondAlias, secondURL),
			fmt.Sprintf("Failed to diff '%s' and '%s'", firstURL, secondURL))
	}

	// Diff first and second urls.
	for diffMsg := range bucketObjectDifference(ctx, firstClient, secondClient) {
		if diffMsg.Error != nil {
			errorIf(diffMsg.Error, "Unable to calculate objects difference.")
			// Ignore error and proceed to next object.
			continue
		}
		printMsg(diffMsg)
	}

	return nil
}

// mainDiff main for 'diff'.
func mainDiff(cliCtx *cli.Context) error {
	ctx, cancelDiff := context.WithCancel(globalContext)
	defer cancelDiff()

	// Parse encryption keys per command.
	encKeyDB, err := validateAndCreateEncryptionKeys(cliCtx)
	fatalIf(err, "Unable to parse encryption keys.")

	// check 'diff' cli arguments.
	checkDiffSyntax(ctx, cliCtx, encKeyDB)

	// Additional command specific theme customization.
	console.SetColor("DiffMessage", color.New(color.FgGreen, color.Bold))
	console.SetColor("DiffOnlyInFirst", color.New(color.FgRed))
	console.SetColor("DiffOnlyInSecond", color.New(color.FgGreen))
	console.SetColor("DiffType", color.New(color.FgMagenta))
	console.SetColor("DiffSize", color.New(color.FgYellow, color.Bold))
	console.SetColor("DiffMetadata", color.New(color.FgYellow, color.Bold))
	console.SetColor("DiffMMSourceMTime", color.New(color.FgYellow, color.Bold))

	URLs := cliCtx.Args()
	firstURL := URLs.Get(0)
	secondURL := URLs.Get(1)

	return doDiffMain(ctx, firstURL, secondURL)
}
