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

var (
	// mcVersion - version time.RFC3339.
	mcVersion = "DEVELOPMENT.GOGET"
	// mcReleaseTag - release tag in TAG.%Y-%m-%dT%H-%M-%SZ.
	mcReleaseTag = "DEVELOPMENT.GOGET"
	// mcCommitID - latest commit id.
	mcCommitID = "DEVELOPMENT.GOGET"
	// mcShortCommitID - first 12 characters from mcCommitID.
	mcShortCommitID = mcCommitID[:12]
)
