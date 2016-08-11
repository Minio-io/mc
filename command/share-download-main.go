/*
 * Minio Client (C) 2014, 2015 Minio, Inc.
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

package command

import (
	"time"

	"github.com/minio/cli"
	"github.com/minio/minio/pkg/probe"
)

var (
	shareDownloadFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "help, h",
			Usage: "Help of share download",
		},
		cli.BoolFlag{
			Name:  "recursive, r",
			Usage: "Share all objects recursively.",
		},
		shareFlagExpire,
	}
)

// Share documents via URL.
var shareDownload = cli.Command{
	Name:   "download",
	Usage:  "Generate URLs for download access.",
	Action: mainShareDownload,
	Flags:  append(shareDownloadFlags, globalFlags...),
	CustomHelpTemplate: `NAME:
   mc share {{.Name}} - {{.Usage}}

USAGE:
   mc share {{.Name}} [OPTIONS] TARGET [TARGET...]

OPTIONS:
  {{range .Flags}}{{.}}
  {{end}}
EXAMPLES:
   1. Share this object with 7 days default expiry.
      $ mc share {{.Name}} s3/backup/2006-Mar-1/backup.tar.gz

   2. Share this object with 10 minutes expiry.
      $ mc share {{.Name}} --expire=10m s3/backup/2006-Mar-1/backup.tar.gz

   3. Share all objects under this folder with 5 days expiry.
      $ mc share {{.Name}} --expire=120h s3/backup/

   4. Share all objects under this folder and all its sub-folders with 5 days expiry.
      $ mc share {{.Name}} --recursive --expire=120h s3/backup/
`,
}

// checkShareDownloadSyntax - validate command-line args.
func checkShareDownloadSyntax(ctx *cli.Context) {
	args := ctx.Args()
	if !args.Present() {
		cli.ShowCommandHelpAndExit(ctx, "download", 1) // last argument is exit code.
	}

	// Parse expiry.
	expiry := shareDefaultExpiry
	expireArg := ctx.String("expire")
	if expireArg != "" {
		var e error
		expiry, e = time.ParseDuration(expireArg)
		fatalIf(probe.NewError(e), "Unable to parse expire=‘"+expireArg+"’.")
	}

	// Validate expiry.
	if expiry.Seconds() < 1 {
		fatalIf(errDummy().Trace(expiry.String()), "Expiry cannot be lesser than 1 second.")
	}
	if expiry.Seconds() > 604800 {
		fatalIf(errDummy().Trace(expiry.String()), "Expiry cannot be larger than 7 days.")
	}

	for _, url := range ctx.Args() {
		_, _, err := url2Stat(url)
		fatalIf(err.Trace(url), "Unable to stat ‘"+url+"’.")
	}
}

// doShareURL share files from target.
func doShareDownloadURL(targetURL string, isRecursive bool, expiry time.Duration) *probe.Error {
	targetAlias, targetURLFull, _, err := expandAlias(targetURL)
	if err != nil {
		return err.Trace(targetURL)
	}
	clnt, err := newClientFromAlias(targetAlias, targetURLFull)
	if err != nil {
		return err.Trace(targetURL)
	}

	// Load previously saved upload-shares. Add new entries and write it back.
	shareDB := newShareDBV1()
	shareDownloadsFile := getShareDownloadsFile()
	err = shareDB.Load(shareDownloadsFile)
	if err != nil {
		return err.Trace(shareDownloadsFile)
	}

	// Generate share URL for each target.
	incomplete := false
	for content := range clnt.List(isRecursive, incomplete) {
		if content.Err != nil {
			return content.Err.Trace(clnt.GetURL().String())
		}
		// if any incoming directories, we don't need to calculate.
		if content.Type.IsDir() {
			continue
		}
		objectURL := content.URL.String()
		newClnt, err := newClientFromAlias(targetAlias, objectURL)
		if err != nil {
			return err.Trace(objectURL)
		}

		// Generate share URL.
		shareURL, err := newClnt.ShareDownload(expiry)
		if err != nil {
			// add objectURL and expiry as part of the trace arguments.
			return err.Trace(objectURL, "expiry="+expiry.String())
		}

		// Make new entries to shareDB.
		contentType := "" // Not useful for download shares.
		shareDB.Set(objectURL, shareURL, expiry, contentType)
		printMsg(shareMesssage{
			ObjectURL:   objectURL,
			ShareURL:    shareURL,
			TimeLeft:    expiry,
			ContentType: contentType,
		})
	}

	// Save downloads and return.
	return shareDB.Save(shareDownloadsFile)
}

// main for share download.
func mainShareDownload(ctx *cli.Context) {
	// Set global flags from context.
	setGlobalsFromContext(ctx)

	// check input arguments.
	checkShareDownloadSyntax(ctx)

	// Initialize share config folder.
	initShareConfig()

	// Additional command speific theme customization.
	shareSetColor()

	// Set command flags from context.
	isRecursive := ctx.Bool("recursive")
	expiry := shareDefaultExpiry
	if ctx.String("expire") != "" {
		var e error
		expiry, e = time.ParseDuration(ctx.String("expire"))
		fatalIf(probe.NewError(e), "Unable to parse expire=‘"+ctx.String("expire")+"’.")
	}

	for _, targetURL := range ctx.Args() {
		err := doShareDownloadURL(targetURL, isRecursive, expiry)
		fatalIf(err.Trace(targetURL), "Unable to share target ‘"+targetURL+"’.")
	}
}
