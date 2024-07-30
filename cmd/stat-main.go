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
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/minio/cli"
	"github.com/minio/pkg/v3/console"
)

// stat specific flags.
var (
	statFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "rewind",
			Usage: "stat on older version(s)",
		},
		cli.BoolFlag{
			Name:  "versions",
			Usage: "stat all versions",
		},
		cli.StringFlag{
			Name:  "version-id, vid",
			Usage: "stat a specific object version",
		},
		cli.BoolFlag{
			Name:  "recursive, r",
			Usage: "stat all objects recursively",
		},
	}
)

// show object metadata
var statCmd = cli.Command{
	Name:         "stat",
	Usage:        "show object metadata",
	Action:       mainStat,
	OnUsageError: onUsageError,
	Before:       setGlobalsFromContext,
	Flags:        append(append(statFlags, encCFlag), globalFlags...),
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} [FLAGS] TARGET [TARGET ...]

FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}

EXAMPLES:
  1. Stat all contents of mybucket on Amazon S3 cloud storage.
     {{.Prompt}} {{.HelpName}} s3/mybucket/

  2. Stat all contents of mybucket on Amazon S3 cloud storage on Microsoft Windows.
     {{.Prompt}} {{.HelpName}} s3\mybucket\

  3. Stat files recursively on a local filesystem on Microsoft Windows.
     {{.Prompt}} {{.HelpName}} --recursive C:\Users\mydocuments\

  4. Stat encrypted files on Amazon S3 cloud storage. In case the encryption key contains non-printable character like tab, pass the
     base64 encoded string as key.
     {{.Prompt}} {{.HelpName}} --enc-c "s3/personal-document/=MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDA" s3/personal-document/2019-account_report.docx

  5. Stat a specific object version.
     {{.Prompt}} {{.HelpName}} --version-id "CL3sWgdSN2pNntSf6UnZAuh2kcu8E8si" s3/personal-docs/2018-account_report.docx

  6. Stat all objects versions recursively created before 1st January 2020.
     {{.Prompt}} {{.HelpName}} --versions --rewind 2020.01.01T00:00 s3/personal-docs/
`,
}

// parseAndCheckStatSyntax - parse and validate all the passed arguments
func parseAndCheckStatSyntax(ctx context.Context, cliCtx *cli.Context, encKeyDB map[string][]prefixSSEPair) ([]string, bool, string, time.Time, bool) {
	if !cliCtx.Args().Present() {
		showCommandHelpAndExit(cliCtx, 1) // last argument is exit code
	}

	args := cliCtx.Args()
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			fatalIf(errInvalidArgument().Trace(args...), "Unable to validate empty argument.")
		}
	}

	recursive := cliCtx.Bool("recursive")
	versionID := cliCtx.String("version-id")
	withVersions := cliCtx.Bool("versions")
	rewind := parseRewindFlag(cliCtx.String("rewind"))

	// extract URLs.
	URLs := cliCtx.Args()

	if versionID != "" && len(args) > 1 {
		fatalIf(errInvalidArgument().Trace(args...), "You cannot specify --version-id with multiple arguments.")
	}

	if versionID != "" && (recursive || withVersions || !rewind.IsZero()) {
		fatalIf(errInvalidArgument().Trace(args...), "You cannot specify --version-id with either --rewind, --versions or --recursive.")
	}
	var targetUrls []string
	for _, url := range URLs {
		_, _, err := url2Stat(ctx, url2StatOptions{urlStr: url, versionID: versionID, fileAttr: false, encKeyDB: encKeyDB, timeRef: rewind, isZip: false, ignoreBucketExistsCheck: false})
		if err != nil {
			fatalIf(err.Trace(url), "Unable to stat `"+url+"`.")
		}
		_, path := url2Alias(url)
		if path != "" {
			targetUrls = append(targetUrls, url)
			continue
		}
		clnt, err := newClient(url)
		fatalIf(err.Trace(args...), "Unable to initialize `"+url+"`.")
		buckets, e := clnt.ListBuckets(ctx)
		if e != nil || len(buckets) == 0 {
			targetUrls = append(targetUrls, url)
			continue
		}
		for _, bucket := range buckets {
			targetUrls = append(targetUrls, filepath.Join(url, bucket.BucketName))
		}
	}
	return targetUrls, recursive, versionID, rewind, withVersions
}

// mainStat - is a handler for mc stat command
func mainStat(cliCtx *cli.Context) error {
	ctx, cancelStat := context.WithCancel(globalContext)
	defer cancelStat()

	// Additional command specific theme customization.
	console.SetColor("Name", color.New(color.Bold, color.FgCyan))
	console.SetColor("Date", color.New(color.FgWhite))
	console.SetColor("Size", color.New(color.FgWhite))
	console.SetColor("ETag", color.New(color.FgWhite))
	console.SetColor("Metadata", color.New(color.FgWhite))
	// theme specific to stat bucket
	console.SetColor("Key", color.New(color.FgCyan))
	console.SetColor("Value", color.New(color.FgYellow))
	console.SetColor("Unset", color.New(color.FgRed))
	console.SetColor("Set", color.New(color.FgGreen))

	console.SetColor("Title", color.New(color.Bold, color.FgBlue))
	console.SetColor("Count", color.New(color.FgGreen))

	// Parse encryption keys per command.
	encKeyDB, err := validateAndCreateEncryptionKeys(cliCtx)
	fatalIf(err, "Unable to parse encryption keys.")

	// check 'stat' cli arguments.
	args, isRecursive, versionID, rewind, withVersions := parseAndCheckStatSyntax(ctx, cliCtx, encKeyDB)
	// mimic operating system tool behavior.
	if len(args) == 0 {
		args = []string{"."}
	}

	for _, targetURL := range args {
		fatalIf(statURL(ctx, targetURL, versionID, rewind, withVersions, false, isRecursive, encKeyDB), "Unable to stat `"+targetURL+"`.")
	}

	return nil
}
