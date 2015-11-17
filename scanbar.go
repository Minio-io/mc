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

package main

import (
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/minio/mc/pkg/console"
	"github.com/minio/minio-xl/pkg/probe"
	"github.com/olekukonko/ts"
)

/******************************** Scan Bar ************************************/
// fixateScanBar truncates long text to fit within the terminal size.
func fixateScanBar(text string, width int) string {
	if len([]rune(text)) > width {
		// Trim text to fit within the screen
		trimSize := len([]rune(text)) - width + 3 //"..."
		if trimSize < len([]rune(text)) {
			text = "..." + text[trimSize:]
		}
	}
	return text
}

// Progress bar function report objects being scaned.
type scanBarFunc func(string)

// scanBarFactory returns a progress bar function to report URL scanning.
func scanBarFactory() scanBarFunc {
	prevLineSize := 0
	prevSource := ""
	fileCount := 0
	termSize, err := ts.GetSize()
	if err != nil {
		fatalIf(probe.NewError(err), "Unable to get terminal size. Please use --quiet option.")
	}
	termWidth := termSize.Col()
	cursorCh := cursorAnimate()

	return func(source string) {
		scanPrefix := fmt.Sprintf("[%s] %s ", humanize.Comma(int64(fileCount)), string(<-cursorCh))
		cmnPrefix := commonPrefix(source, prevSource)
		eraseLen := prevLineSize - len([]rune(scanPrefix+cmnPrefix))
		if eraseLen < 1 {
			eraseLen = 0
		}
		if prevLineSize != 0 { // erase previous line
			console.PrintC("\r" + scanPrefix + cmnPrefix + strings.Repeat(" ", eraseLen))
		}

		source = fixateScanBar(source, termWidth-len([]rune(scanPrefix))-1)
		barText := scanPrefix + source
		console.PrintC("\r" + barText)
		prevSource = source
		prevLineSize = len([]rune(barText))
		fileCount++
	}
}
