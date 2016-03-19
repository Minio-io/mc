/*
 * Minio Client (C) 2015, 2016 Minio, Inc.
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

	"github.com/minio/cli"
	"github.com/minio/minio/pkg/probe"
)

type mirrorURLs struct {
	SourceAlias   string
	SourceContent *clientContent
	TargetAlias   string
	TargetContent *clientContent
	Error         *probe.Error `json:"-"`
}

func (m mirrorURLs) isEmpty() bool {
	if m.SourceContent == nil && m.TargetContent == nil && m.Error == nil {
		return true
	}
	if m.SourceContent.Size == 0 && m.TargetContent == nil && m.Error == nil {
		return true
	}
	return false
}

//
//   * MIRROR ARGS - VALID CASES
//   =========================
//   mirror(d1..., d2) -> []mirror(d1/f, d2/d1/f)

// checkMirrorSyntax(URLs []string)
func checkMirrorSyntax(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		cli.ShowCommandHelpAndExit(ctx, "mirror", 1) // last argument is exit code.
	}

	// extract URLs.
	URLs := ctx.Args()
	srcURL := URLs[0]
	tgtURL := URLs[1]

	/****** Generic rules *******/
	_, srcContent, err := url2Stat(srcURL)
	// incomplete uploads are not necessary for copy operation, no need to verify for them.
	isIncomplete := false
	if err != nil && !isURLPrefixExists(srcURL, isIncomplete) {
		fatalIf(err.Trace(srcURL), "Unable to stat source ‘"+srcURL+"’.")
	}

	if err == nil && !srcContent.Type.IsDir() {
		fatalIf(errInvalidArgument().Trace(srcContent.URL.String(), srcContent.Type.String()), fmt.Sprintf("Source ‘%s’ is not a folder. Only folders are supported by mirror command.", srcURL))
	}

	if len(tgtURL) == 0 && tgtURL == "" {
		fatalIf(errInvalidArgument().Trace(), "Invalid target arguments to mirror command.")
	}

	url := newURL(tgtURL)
	if url.Host != "" {
		if !isURLVirtualHostStyle(url.Host) {
			if url.Path == string(url.Separator) {
				fatalIf(errInvalidArgument().Trace(tgtURL),
					fmt.Sprintf("Target ‘%s’ does not contain bucket name.", tgtURL))
			}
		}
	}
	_, _, err = url2Stat(tgtURL)
	// we die on any error other than PathNotFound - destination directory need not exist.
	if _, ok := err.ToGoError().(PathNotFound); !ok {
		fatalIf(err.Trace(tgtURL), fmt.Sprintf("Unable to stat %s", tgtURL))
	}
}

func deltaSourceTargets(sourceURL string, targetURL string, isForce bool, isFake bool, isRemove bool, mirrorURLsCh chan<- mirrorURLs) {
	defer close(mirrorURLsCh)

	// source and targets are always directories
	sourceSeparator := string(newURL(sourceURL).Separator)
	if !strings.HasSuffix(sourceURL, sourceSeparator) {
		sourceURL = sourceURL + sourceSeparator
	}
	targetSeparator := string(newURL(targetURL).Separator)
	if !strings.HasSuffix(targetURL, targetSeparator) {
		targetURL = targetURL + targetSeparator
	}

	// Extract alias and expanded URL
	sourceAlias, sourceURL, _ := mustExpandAlias(sourceURL)
	targetAlias, targetURL, _ := mustExpandAlias(targetURL)

	sourceClient, err := newClientFromAlias(sourceAlias, sourceURL)
	if err != nil {
		mirrorURLsCh <- mirrorURLs{Error: err.Trace(sourceAlias, sourceURL)}
		return
	}

	targetClient, err := newClientFromAlias(targetAlias, targetURL)
	if err != nil {
		mirrorURLsCh <- mirrorURLs{Error: err.Trace(targetAlias, targetURL)}
		return
	}

	for d := range differenceCh(sourceClient, targetClient) {
		if d.err != nil {
			mirrorURLsCh <- mirrorURLs{
				Error: d.err.Trace(sourceURL, targetURL),
			}
			continue
		}
		differ := d.dType
		if isRemove && isForce && differ == differInTime || differ == differInSize {
			// Do not attempt to list files for removal if they happen
			// to differ in time and size.
			continue
		}
		if isRemove && isForce && differ == differInSecond {
			// mirrorURLsCh <- mirrorURLs{}
			continue
		}
		if differ == differInNone || differ == differInSecond {
			// No difference, continue.
			continue
		}
		if differ == differInType {
			mirrorURLsCh <- mirrorURLs{Error: errInvalidTarget(d.targetURL)}
			continue
		}
		if differ == differInSize && !isForce && !isFake {
			// Size differs and force not set
			mirrorURLsCh <- mirrorURLs{Error: errOverWriteNotAllowed(d.sourceURL)}
			continue
		}
		// either available only in source or size differs and force is set
		sourceContent := d.sourceContent
		sourceSuffix := strings.TrimPrefix(sourceContent.URL.String(), sourceURL)
		targetContent := &clientContent{URL: *newURL(urlJoinPath(targetURL, sourceSuffix))}
		mirrorURLsCh <- mirrorURLs{
			SourceAlias:   sourceAlias,
			SourceContent: sourceContent,
			TargetAlias:   targetAlias,
			TargetContent: targetContent,
		}
	}
}

func prepareMirrorURLs(sourceURL string, targetURL string, isForce bool, isFake bool, isRemove bool) <-chan mirrorURLs {
	mirrorURLsCh := make(chan mirrorURLs)
	go deltaSourceTargets(sourceURL, targetURL, isForce, isFake, isRemove, mirrorURLsCh)
	return mirrorURLsCh
}
