/*
 * MinIO Client (C) 2019 MinIO, Inc.
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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/minio/cli"
	"github.com/minio/mc/pkg/console"
	"github.com/minio/mc/pkg/probe"
)

const (
	treeEntry     = "├─ "
	treeLastEntry = "└─ "
	treeNext      = "│"
	treeLevel     = "  "
)

// Structured message depending on the type of console.
type treeMessage struct {
	Entry        string
	IsDir        bool
	BranchString string
}

// Colorized message for console printing.
func (t treeMessage) String() string {
	entryType := "File"
	if t.IsDir {
		entryType = "Dir"
	}
	return fmt.Sprintf("%s%s", t.BranchString, console.Colorize(entryType, t.Entry))
}

// JSON'ified message for scripting.
// Does No-op. JSON requests are redirected to `ls -r --json`
func (t treeMessage) JSON() string {
	fatalIf(probe.NewError(errors.New("JSON() should never be called here")), "Unable to list in tree format. Please report this issue at https://github.com/minio/mc/issues")
	return ""
}

var treeFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "files, f",
		Usage: "includes files in tree",
	},
	cli.IntFlag{
		Name:  "depth, d",
		Usage: "sets the depth threshold",
		Value: -1,
	},
}

// trees files and folders.
var treeCmd = cli.Command{
	Name:   "tree",
	Usage:  "list buckets and objects in a tree format",
	Action: mainTree,
	Before: setGlobalsFromContext,
	Flags:  append(treeFlags, globalFlags...),
	CustomHelpTemplate: `NAME:
  {{.HelpName}} - {{.Usage}}

USAGE:
  {{.HelpName}} [FLAGS] TARGET [TARGET ...]

FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}
EXAMPLES:
   1. List all buckets and directories on MinIO object storage server in tree format.
      $ {{.HelpName}} myminio

   2. List all directories in "mybucket" on MinIO object storage server in tree format.
      $ {{.HelpName}} myminio/mybucket/

   3. List all directories in "mybucket" on MinIO object storage server hosted on Microsoft Windows in tree format.
      $ {{.HelpName}} myminio\mybucket\

   4. List all directories and objects in "mybucket" on MinIO object storage server in tree format.
      $ {{.HelpName}} --files myminio/mybucket/

   5. List all directories upto depth level '2' in tree format.
      $ {{.HelpName}} --depth 2 myminio/mybucket/
`,
}

// checkTreeSyntax - validate all the passed arguments
func checkTreeSyntax(ctx *cli.Context) {
	args := ctx.Args()

	if ctx.IsSet("depth") {
		if ctx.Int("depth") < -1 || ctx.Int("depth") == 0 {
			fatalIf(errInvalidArgument().Trace(args...), "please set a proper depth, for example: '--depth 1' to limit the tree output, default (-1) output displays everything")
		}
	}

	if (args.Present()) && len(args) == 0 {
		return
	}

	for _, url := range args {
		if _, _, err := url2Stat(url, false, nil); err != nil && !isURLPrefixExists(url, false) {
			fatalIf(err.Trace(url), "Unable to tree `"+url+"`.")
		}
	}
}

// doTree - list all entities inside a folder in a tree format.
func doTree(url string, level int, leaf bool, branchString string, depth int, includeFiles bool) error {

	targetAlias, targetURL, _ := mustExpandAlias(url)
	if !strings.HasSuffix(targetURL, "/") {
		targetURL += "/"
	}

	clnt, err := newClientFromAlias(targetAlias, targetURL)
	fatalIf(err.Trace(targetURL), "Unable to initialize target `"+targetURL+"`.")

	prefixPath := clnt.GetURL().Path
	separator := string(clnt.GetURL().Separator)
	if !strings.HasSuffix(prefixPath, separator) {
		prefixPath = filepath.Dir(prefixPath) + "/"
	}

	bucketNameShowed := false
	var prev *clientContent
	show := func(end bool) error {
		currbranchString := branchString
		if level == 1 && !bucketNameShowed {
			bucketNameShowed = true
			printMsg(treeMessage{
				Entry:        url,
				IsDir:        true,
				BranchString: branchString,
			})
		}

		isLevelClosed := strings.HasSuffix(currbranchString, treeLastEntry)
		if isLevelClosed {
			currbranchString = strings.TrimSuffix(currbranchString, treeLastEntry)
		} else {
			currbranchString = strings.TrimSuffix(currbranchString, treeEntry)
		}

		if level != 1 {
			if isLevelClosed {
				currbranchString += " " + treeLevel
			} else {
				currbranchString += treeNext + treeLevel
			}
		}

		if end {
			currbranchString += treeLastEntry
		} else {
			currbranchString += treeEntry
		}

		// Convert any os specific delimiters to "/".
		contentURL := filepath.ToSlash(prev.URL.Path)
		prefixPath = filepath.ToSlash(prefixPath)

		// Trim prefix of current working dir
		prefixPath = strings.TrimPrefix(prefixPath, "."+separator)

		if prev.Type.IsDir() {
			printMsg(treeMessage{
				Entry:        strings.TrimSuffix(strings.TrimPrefix(contentURL, prefixPath), "/"),
				IsDir:        true,
				BranchString: currbranchString,
			})
		} else {
			printMsg(treeMessage{
				Entry:        strings.TrimPrefix(contentURL, prefixPath),
				IsDir:        false,
				BranchString: currbranchString,
			})
		}

		if prev.Type.IsDir() {
			url := ""
			if targetAlias != "" {
				url = targetAlias + "/" + contentURL
			} else {
				url = contentURL
			}

			if depth == -1 || level <= depth {
				if err := doTree(url, level+1, end, currbranchString, depth, includeFiles); err != nil {
					return err
				}
			}
		}

		return nil
	}

	for content := range clnt.List(false, false, DirNone) {

		if !includeFiles && !content.Type.IsDir() {
			continue
		}

		if content.Err != nil {
			errorIf(content.Err.Trace(clnt.GetURL().String()), "Unable to tree.")
			continue
		}

		if prev != nil {
			if err := show(false); err != nil {
				return err
			}
		}

		prev = content
	}

	if prev != nil {
		if err := show(true); err != nil {
			return err
		}
	}

	return nil
}

// mainTree - is a handler for mc tree command
func mainTree(ctx *cli.Context) error {

	// check 'tree' cli arguments.
	checkTreeSyntax(ctx)

	console.SetColor("File", color.New(color.Bold))
	console.SetColor("Dir", color.New(color.FgCyan, color.Bold))

	args := ctx.Args()
	// mimic operating system tool behavior.
	if !ctx.Args().Present() {
		args = []string{"."}
	}

	includeFiles := ctx.Bool("files")
	depth := ctx.Int("depth")

	var cErr error
	for _, targetURL := range args {
		if !globalJSON {
			if e := doTree(targetURL, 1, false, "", depth, includeFiles); e != nil {
				cErr = e
			}
		} else {
			targetAlias, targetURL, _ := mustExpandAlias(targetURL)
			if !strings.HasSuffix(targetURL, "/") {
				targetURL += "/"
			}
			clnt, err := newClientFromAlias(targetAlias, targetURL)
			fatalIf(err.Trace(targetURL), "Unable to initialize target `"+targetURL+"`.")
			if e := doList(clnt, true, false); e != nil {
				cErr = e
			}
		}
	}
	return cErr
}
