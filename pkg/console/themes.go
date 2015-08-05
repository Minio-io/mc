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

package console

import "github.com/fatih/color"

// MiniTheme - Minio's default color theme
var MiniTheme = Theme{
	Debug:     (color.New(color.FgWhite, color.Faint, color.Italic)),
	Fatal:     (color.New(color.FgRed, color.Italic, color.Bold)),
	Error:     (color.New(color.FgYellow, color.Italic)),
	Info:      (color.New(color.FgGreen, color.Bold)),
	File:      (color.New(color.FgWhite)),
	Dir:       (color.New(color.FgCyan, color.Bold)),
	Command:   (color.New(color.FgWhite, color.Bold)),
	SessionID: (color.New(color.FgYellow, color.Bold)),
	Size:      (color.New(color.FgYellow)),
	Time:      (color.New(color.FgGreen)),
	JSON:      (color.New(color.FgWhite, color.Italic)),
	Bar:       (color.New(color.FgGreen, color.Bold)),
	PrintC:    (color.New(color.FgGreen, color.Bold)),
	Print:     (color.New()),
}

// WhiteTheme - All white color theme
var WhiteTheme = Theme{
	Debug:     (color.New(color.FgWhite, color.Faint, color.Italic)),
	Fatal:     (color.New(color.FgWhite, color.Bold, color.Italic)),
	Error:     (color.New(color.FgWhite, color.Bold, color.Italic)),
	Info:      (color.New(color.FgWhite, color.Bold)),
	File:      (color.New(color.FgWhite, color.Bold)),
	Dir:       (color.New(color.FgWhite, color.Bold)),
	Command:   (color.New(color.FgWhite, color.Bold)),
	SessionID: (color.New(color.FgWhite, color.Bold)),
	Size:      (color.New(color.FgWhite, color.Bold)),
	Time:      (color.New(color.FgWhite, color.Bold)),
	JSON:      (color.New(color.FgWhite, color.Bold, color.Italic)),
	Bar:       (color.New(color.FgWhite, color.Bold)),
	PrintC:    (color.New(color.FgWhite, color.Bold)),
	Print:     (color.New()),
}

// NoColorTheme - Disables color theme
var NoColorTheme = Theme{
	Debug:     new(color.Color),
	Fatal:     new(color.Color),
	Error:     new(color.Color),
	Info:      new(color.Color),
	File:      new(color.Color),
	Dir:       new(color.Color),
	Command:   new(color.Color),
	SessionID: new(color.Color),
	Size:      new(color.Color),
	Time:      new(color.Color),
	JSON:      new(color.Color),
	Bar:       new(color.Color),
	PrintC:    new(color.Color),
	Print:     new(color.Color),
}
