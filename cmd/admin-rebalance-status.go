// Copyright (c) 2022 MinIO, Inc.
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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/minio/cli"
	"github.com/minio/mc/pkg/probe"
	"github.com/minio/pkg/console"
)

var adminRebalanceStatusCmd = cli.Command{
	Name:         "status",
	Usage:        "Show status of an ongoing rebalance operation",
	Action:       mainAdminRebalanceStatus,
	OnUsageError: onUsageError,
	Before:       setGlobalsFromContext,
	Flags:        globalFlags,
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} ALIAS

FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}

EXAMPLES:
  1. Fetch status of an ongoing rebalance on a MinIO deployment with alias myminio
     {{.Prompt}} {{.HelpName}} myminio
`,
}

func mainAdminRebalanceStatus(ctx *cli.Context) error {
	if len(ctx.Args()) != 1 {
		cli.ShowCommandHelpAndExit(ctx, "rebalance", 1)
	}

	args := ctx.Args()
	aliasedURL := args.Get(0)

	var pErr *probe.Error
	client, pErr := newAdminClient(aliasedURL)
	if pErr != nil {
		fatalIf(pErr.Trace(aliasedURL), "Unable to initialize admin client")
		return pErr.ToGoError()
	}

	rInfo, err := client.RebalanceStatus(globalContext)
	if err != nil {
		fatalIf(probe.NewError(err), "Failed to get rebalance status")
	}
	if globalJSON {
		b, err := json.Marshal(rInfo)
		fatalIf(probe.NewError(err), "Failed to marshal json")
		console.Println(string(b))
		return nil
	}

	// col-headers
	colHeaders := make([]string, len(rInfo.Pools))
	for i := range rInfo.Pools {
		colHeaders[i] = fmt.Sprintf("Pool-%d", i)
	}
	var (
		totalBytes, totalObjects, totalVersions uint64
		maxElapsed, maxETA                      time.Duration
	)
	row := make([]string, len(rInfo.Pools))
	for idx, pool := range rInfo.Pools {
		statusStr := fmt.Sprintf("%.2f%%", pool.Used)
		if pool.Status == "Started" {
			statusStr += " *" // indicating rebalance is in progress in this pool
		}
		row[idx] = statusStr

		// For summary values
		totalBytes += pool.Progress.Bytes
		totalObjects += pool.Progress.NumObjects
		totalVersions += pool.Progress.NumVersions
		if maxElapsed == 0 || maxElapsed < pool.Progress.Elapsed {
			maxElapsed = pool.Progress.Elapsed
		}
		if maxETA == 0 || maxETA < pool.Progress.ETA {
			maxETA = pool.Progress.ETA
		}
	}

	dspOrder := []col{colGreen, colGrey}
	var printColors []*color.Color
	for _, c := range dspOrder {
		printColors = append(printColors, getPrintCol(c))
	}
	alignRights := make([]bool, len(rInfo.Pools))
	tbl := console.NewTable(printColors, alignRights, 0)
	err = tbl.DisplayTable([][]string{colHeaders, row})
	if err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Summary: \n")
	fmt.Fprintf(&b, "Data: %s (%d objects, %d versions) \n", humanize.Bytes(totalBytes), totalObjects, totalVersions)
	fmt.Fprintf(&b, "Time: %s (%s to completion)", maxElapsed, maxETA)
	console.Println(b.String())
	return nil
}
