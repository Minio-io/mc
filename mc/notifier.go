/*
 * Minio Client (C) 2015 Minio, Inc.
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

package mc

import (
	"fmt"
	"math"
	"runtime"
	"strings"

	"github.com/cheggaaa/pb"
	"github.com/fatih/color"
	"github.com/minio/minio/pkg/probe"
)

// colorizeUpdateMessage - inspired from Yeoman project npm package https://github.com/yeoman/update-notifier.
func colorizeUpdateMessage(updateString string) (string, *probe.Error) {
	// initialize coloring
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow, color.Bold).SprintfFunc()

	// calculate length without color coding, due to ANSI color characters padded to actual
	// string the final length is wrong than the original string length.
	line1Str := fmt.Sprintf("  New update available, please execute the following command to update: ")
	line2Str := fmt.Sprintf("  %s ", updateString)
	line1Length := len(line1Str)
	line2Length := len(line2Str)

	// populate lines with color coding.
	line1InColor := line1Str
	line2InColor := fmt.Sprintf("  %s ", cyan(updateString))

	// calculate the rectangular box size.
	maxContentWidth := int(math.Max(float64(line1Length), float64(line2Length)))
	line1Rest := maxContentWidth - line1Length
	line2Rest := maxContentWidth - line2Length

	termWidth, e := pb.GetTerminalWidth()
	if e != nil {
		return "", probe.NewError(e)
	}
	var message string
	switch {
	case len(line2Str) > termWidth:
		message = "\n" + line1InColor + "\n" + line2InColor + "\n"
	default:
		// on windows terminal turn off unicode characters.
		var top, bottom, sideBar string
		if runtime.GOOS == "windows" {
			top = yellow("*" + strings.Repeat("*", maxContentWidth) + "*")
			bottom = yellow("*" + strings.Repeat("*", maxContentWidth) + "*")
			sideBar = yellow("|")
		} else {
			// color the rectangular box, use unicode characters here.
			top = yellow("┏" + strings.Repeat("━", maxContentWidth) + "┓")
			bottom = yellow("┗" + strings.Repeat("━", maxContentWidth) + "┛")
			sideBar = yellow("┃")
		}
		// fill spaces to the rest of the area.
		spacePaddingLine1 := strings.Repeat(" ", line1Rest)
		spacePaddingLine2 := strings.Repeat(" ", line2Rest)

		// construct the final message.
		message = "\n" + top + "\n" +
			sideBar + line1InColor + spacePaddingLine1 + sideBar + "\n" +
			sideBar + line2InColor + spacePaddingLine2 + sideBar + "\n" +
			bottom + "\n"
	}
	// return the final message
	return message, nil
}
