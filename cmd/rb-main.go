// Copyright (c) 2015-2021 MinIO, Inc.
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

	"github.com/fatih/color"
	"github.com/minio/cli"
	json "github.com/minio/colorjson"
	"github.com/minio/mc/pkg/probe"
	"github.com/minio/minio-go/v7"
	"github.com/minio/pkg/console"
)

var (
	rbFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "force",
			Usage: "force a recursive remove operation on all object versions",
		},
		cli.BoolFlag{
			Name:  "dangerous",
			Usage: "allow **site-wide** removal of objects",
		},
	}
)

// remove a bucket.
var rbCmd = cli.Command{
	Name:         "rb",
	Usage:        "remove a bucket",
	Action:       mainRemoveBucket,
	OnUsageError: onUsageError,
	Before:       setGlobalsFromContext,
	Flags:        append(rbFlags, globalFlags...),
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} [FLAGS] TARGET [TARGET...]
{{if .VisibleFlags}}
FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}{{end}}
EXAMPLES:
  1. Remove an empty bucket on Amazon S3 cloud storage
     {{.Prompt}} {{.HelpName}} s3/mybucket

  2. Remove a directory hierarchy.
     {{.Prompt}} {{.HelpName}} /tmp/this/new/dir1

  3. Remove bucket 'jazz-songs' and all its contents
     {{.Prompt}} {{.HelpName}} --force s3/jazz-songs

  4. Remove all buckets and objects recursively from S3 host
     {{.Prompt}} {{.HelpName}} --force --dangerous s3
`,
}

// removeBucketMessage is container for delete bucket success and failure messages.
type removeBucketMessage struct {
	Status string `json:"status"`
	Bucket string `json:"bucket"`
}

// String colorized delete bucket message.
func (s removeBucketMessage) String() string {
	return console.Colorize("RemoveBucket", fmt.Sprintf("Removed `%s` successfully.", s.Bucket))
}

// JSON jsonified remove bucket message.
func (s removeBucketMessage) JSON() string {
	removeBucketJSONBytes, e := json.Marshal(s)
	fatalIf(probe.NewError(e), "Unable to marshal into JSON.")

	return string(removeBucketJSONBytes)
}

// Validate command line arguments.
func checkRbSyntax(ctx context.Context, cliCtx *cli.Context) {
	if !cliCtx.Args().Present() {
		exitCode := 1
		cli.ShowCommandHelpAndExit(cliCtx, "rb", exitCode)
	}
	// Set command flags from context.
	isForce := cliCtx.Bool("force")
	isDangerous := cliCtx.Bool("dangerous")

	for _, url := range cliCtx.Args() {
		if isNamespaceRemoval(ctx, url) {
			if isForce && isDangerous {
				continue
			}
			fatalIf(errDummy().Trace(),
				"This operation results in **site-wide** removal of buckets. If you are really sure, retry this command with ‘--force’ and ‘--dangerous’ flags.")
		}
	}
}

// Delete a bucket and all its objects and versions will be removed as well.
func deleteBucket(ctx context.Context, url string, isForce bool) *probe.Error {
	targetAlias, targetURL, _ := mustExpandAlias(url)
	clnt, err := newClientFromAlias(targetAlias, targetURL)
	if err != nil {
		return err.Trace(targetAlias, targetURL)
	}
	contentCh := make(chan *ClientContent)
	errorCh := clnt.Remove(ctx, false, false, false, contentCh)

	go func() {
		defer close(contentCh)
		opts := ListOptions{
			Recursive:         true,
			WithOlderVersions: true,
			WithDeleteMarkers: true,
			ShowDir:           DirLast,
		}

		for content := range clnt.List(ctx, opts) {
			if content.Err != nil {
				contentCh <- content
				continue
			}

			select {
			case contentCh <- content:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Give up on the first error.
	for perr := range errorCh {
		return perr
	}

	// Remove a bucket without force flag first because force
	// won't work if a bucket has some locking rules, that's
	// why we start with regular bucket removal first.
	if err = clnt.RemoveBucket(ctx, false); err != nil {
		if isForce && minio.ToErrorResponse(err.ToGoError()).Code == "BucketNotEmpty" {
			return clnt.RemoveBucket(ctx, true)
		}
	}
	return err
}

// isNamespaceRemoval returns true if alias
// is not qualified by bucket
func isNamespaceRemoval(ctx context.Context, alias string) bool {
	clnt, err := newClient(alias)
	if err != nil {
		fatalIf(err.Trace(alias), "Unable to initialize")
	}
	return (clnt.GetURL().Path == "/" || clnt.GetURL().Path == "") && clnt.GetURL().Type == objectStorage
}

// mainRemoveBucket is entry point for rb command.
func mainRemoveBucket(cliCtx *cli.Context) error {
	ctx, cancelRemoveBucket := context.WithCancel(globalContext)
	defer cancelRemoveBucket()

	// check 'rb' cli arguments.
	checkRbSyntax(ctx, cliCtx)
	isForce := cliCtx.Bool("force")

	// Additional command specific theme customization.
	console.SetColor("RemoveBucket", color.New(color.FgGreen, color.Bold))

	var cErr error
	for _, targetURL := range cliCtx.Args() {
		// Instantiate client for URL.
		clnt, err := newClient(targetURL)
		if err != nil {
			errorIf(err.Trace(targetURL), "Invalid target `"+targetURL+"`.")
			cErr = exitStatus(globalErrorExitStatus)
			continue
		}

		_, err = clnt.Stat(ctx, StatOptions{})
		if err != nil {
			switch err.ToGoError().(type) {
			case BucketNameEmpty:
			default:
				errorIf(err.Trace(targetURL), "Unable to validate target `"+targetURL+"`.")
				cErr = exitStatus(globalErrorExitStatus)
				continue

			}
		}

		// Check if the bucket contains any object, version or delete marker.
		isEmpty := true
		opts := ListOptions{
			Recursive:         true,
			ShowDir:           DirNone,
			WithOlderVersions: true,
			WithDeleteMarkers: true,
			Count:             1,
		}

		listCtx, listCancel := context.WithCancel(ctx)
		for obj := range clnt.List(listCtx, opts) {
			if obj.Err != nil {
				continue
			}
			isEmpty = false
			break
		}
		listCancel()

		// For all recursive operations make sure to check for 'force' flag.
		if !isForce && !isEmpty {
			fatalIf(errDummy().Trace(), "`"+targetURL+"` is not empty. Retry this command with ‘--force’ flag if you want to remove `"+targetURL+"` and all its contents")
		}

		if isNamespaceRemoval(ctx, targetURL) {
			listCtx, listCancel = context.WithCancel(ctx)
			for obj := range clnt.List(listCtx, ListOptions{
				Recursive: false,
				ShowDir:   DirLast,
			}) {
				if obj.Err != nil {
					continue
				}
				ntargetURL := urlJoinPath(targetURL, obj.URL.String())
				fatalIf(deleteBucket(ctx, ntargetURL, isForce).Trace(ntargetURL), "Failed to remove `"+ntargetURL+"`.")

				printMsg(removeBucketMessage{
					Bucket: ntargetURL, Status: "success",
				})

			}
			listCancel()
		} else {
			fatalIf(deleteBucket(ctx, targetURL, isForce).Trace(targetURL), "Failed to remove `"+targetURL+"`.")

			printMsg(removeBucketMessage{
				Bucket: targetURL, Status: "success",
			})
		}
	}
	return cErr
}
