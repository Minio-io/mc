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
	"sync"

	"github.com/fatih/color"
	"github.com/minio/cli"
	"github.com/minio/mc/pkg/probe"
	"github.com/minio/pkg/v3/console"
)

// mv command flags.
var (
	mvFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "recursive, r",
			Usage: "move recursively",
		},
		cli.StringFlag{
			Name:  "older-than",
			Usage: "move objects older than value in duration string (e.g. 7d10h31s)",
		},
		cli.StringFlag{
			Name:  "newer-than",
			Usage: "move objects newer than value in duration string (e.g. 7d10h31s)",
		},
		cli.StringFlag{
			Name:  "storage-class, sc",
			Usage: "set storage class for new object(s) on target",
		},
		cli.StringFlag{
			Name:  "attr",
			Usage: "add custom metadata for the object",
		},
		cli.BoolFlag{
			Name:  "preserve, a",
			Usage: "preserve filesystem attributes (mode, ownership, timestamps)",
		},
		cli.BoolFlag{
			Name:  "disable-multipart",
			Usage: "disable multipart upload feature",
		},
		cli.StringFlag{
			Name:  "tags",
			Usage: "apply one or more tags to the uploaded objects",
		},
		checksumFlag,
	}
)

// Move command.
var mvCmd = cli.Command{
	Name:         "mv",
	Usage:        "move objects",
	Action:       mainMove,
	OnUsageError: onUsageError,
	Before:       setGlobalsFromContext,
	Flags:        append(append(mvFlags, encFlags...), globalFlags...),
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} [FLAGS] SOURCE [SOURCE...] TARGET

FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}

ENVIRONMENT VARIABLES:
  MC_ENC_KMS: KMS encryption key in the form of (alias/prefix=key).
  MC_ENC_S3: S3 encryption key in the form of (alias/prefix=key).

EXAMPLES:
  01. Move a list of objects from local file system to Amazon S3 cloud storage.
      {{.Prompt}} {{.HelpName}} Music/*.ogg s3/jukebox/

  02. Move a folder recursively from MinIO cloud storage to Amazon S3 cloud storage.
      {{.Prompt}} {{.HelpName}} --recursive play/mybucket/ s3/mybucket/

  03. Move multiple local folders recursively to MinIO cloud storage.
      {{.Prompt}} {{.HelpName}} --recursive backup/2014/ backup/2015/ play/archive/

  04. Move a bucket recursively from aliased Amazon S3 cloud storage to local filesystem on Windows.
      {{.Prompt}} {{.HelpName}} --recursive s3\documents\2014\ C:\Backups\2014

  05. Move files older than 7 days and 10 hours from MinIO cloud storage to Amazon S3 cloud storage.
      {{.Prompt}} {{.HelpName}} --older-than 7d10h play/mybucket/myfolder/ s3/mybucket/

  06. Move files newer than 7 days and 10 hours from MinIO cloud storage to a local path.
      {{.Prompt}} {{.HelpName}} --newer-than 7d10h play/mybucket/myfolder/ ~/latest/

  07. Move an object with name containing unicode characters to Amazon S3 cloud storage.
      {{.Prompt}} {{.HelpName}} 本語 s3/andoria/

  08. Move a local folder with space separated characters to Amazon S3 cloud storage.
      {{.Prompt}} {{.HelpName}} --recursive 'workdir/documents/May 2014/' s3/miniocloud

  09. Move a list of objects from local file system to MinIO cloud storage with specified metadata, separated by ";"
      {{.Prompt}} {{.HelpName}} --attr "key1=value1;key2=value2" Music/*.mp4 play/mybucket/

  10. Move a list of objects from local file system to MinIO cloud storage and set tags to the uploaded objects
      {{.Prompt}} {{.HelpName}} --tag "key1=value1" Music/*.mp4 play/mybucket/

  11. Move a folder recursively from MinIO cloud storage to Amazon S3 cloud storage with Cache-Control and custom metadata, separated by ";".
      {{.Prompt}} {{.HelpName}} --attr "Cache-Control=max-age=90000,min-fresh=9000;key1=value1;key2=value2" --recursive play/mybucket/myfolder/ s3/mybucket/

  12. Move a text file to an object storage and assign REDUCED_REDUNDANCY storage-class to the uploaded object.
      {{.Prompt}} {{.HelpName}} --storage-class REDUCED_REDUNDANCY myobject.txt play/mybucket

  13. Move a text file to an object storage and preserve the file system attribute as metadata.
      {{.Prompt}} {{.HelpName}} -a myobject.txt play/mybucket

  14. Move a text file to an object storage and disable multipart upload feature.
      {{.Prompt}} {{.HelpName}} --disable-multipart myobject.txt play/mybucket

  15. Move a folder using client provided encryption keys from Amazon S3 to MinIO cloud storage.
      {{.Prompt}} {{.HelpName}} --r --enc-c "s3/documents/=MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MBB" --enc-c "myminio/documents/=MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDA" s3/documents/ myminio/documents/

  16. Move a folder using specific server managed encryption keys from Amazon S3 to MinIO cloud storage.
      {{.Prompt}} {{.HelpName}} --r --enc-s3 "s3/documents" --enc-s3 "myminio/documents" s3/documents/ myminio/documents/

  17. Add SHA256 checksum to move a text file to MinIO cloud storage.
      {{.Prompt}} {{.HelpName}} --checksum SHA256 myobject.txt play/mybucket
`,
}

type removeClientInfo struct {
	client    Client
	contentCh chan *ClientContent
	resultCh  <-chan RemoveResult
}

type removeManager struct {
	removeMap      map[string]*removeClientInfo
	removeMapMutex sync.RWMutex
	wg             sync.WaitGroup
}

func (rm *removeManager) readErrors(resultCh <-chan RemoveResult, targetURL string) {
	rm.wg.Add(1)
	go func() {
		defer rm.wg.Done()
		for result := range resultCh {
			if result.Err != nil {
				errorIf(result.Err.Trace(targetURL), "Failed to remove in `%s`.", targetURL)
			}
		}
	}()
}

// This function should be parallel-safe because it is executed by ParallelManager
// If targetAlias is empty, it means we will target local FS contents
func (rm *removeManager) add(ctx context.Context, targetAlias, targetURL string) {
	rm.removeMapMutex.Lock()
	clientInfo := rm.removeMap[targetAlias]
	if clientInfo == nil {
		client, pErr := newClientFromAlias(targetAlias, targetURL)
		if pErr != nil {
			errorIf(pErr.Trace(targetURL), "Invalid argument `%s`.", targetURL)
			return
		}

		contentCh := make(chan *ClientContent, 10000)
		resultCh := client.Remove(ctx, false, false, false, false, contentCh)
		rm.readErrors(resultCh, targetURL)

		clientInfo = &removeClientInfo{
			client:    client,
			contentCh: contentCh,
			resultCh:  resultCh,
		}

		rm.removeMap[targetAlias] = clientInfo
	}
	rm.removeMapMutex.Unlock()

	clientInfo.contentCh <- &ClientContent{URL: *newClientURL(targetURL)}
}

func (rm *removeManager) close() {
	for _, clientInfo := range rm.removeMap {
		close(clientInfo.contentCh)
	}

	// Wait until all on-going client.Remove() operations to finish
	rm.wg.Wait()
}

var rmManager = &removeManager{
	removeMap: make(map[string]*removeClientInfo),
}

// mainMove is the entry point for mv command.
func mainMove(cliCtx *cli.Context) error {
	ctx, cancelMove := context.WithCancel(globalContext)
	defer cancelMove()

	checkCopySyntax(cliCtx)
	console.SetColor("Copy", color.New(color.FgGreen, color.Bold))

	if cliCtx.NArg() == 2 {
		args := cliCtx.Args()
		srcURL := args.Get(0)
		dstURL := args.Get(1)
		if srcURL == dstURL {
			fatalIf(errDummy().Trace(), fmt.Sprintf("Source and destination urls cannot be the same: %v.", srcURL))
		}
	}

	var err *probe.Error

	encKeyDB, err := validateAndCreateEncryptionKeys(cliCtx)
	fatalIf(err, "Unable to parse encryption keys.")

	e := doCopySession(ctx, cancelMove, cliCtx, encKeyDB, true)

	console.Colorize("Copy", "Waiting for move operations to complete")
	rmManager.close()

	return e
}
