/*
 * Mini Object Storage, (C) 2014,2015 Minio, Inc.
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
	"log"
	"os"
	"os/user"

	"github.com/codegangsta/cli"
)

func init() {
	// Check for the environment early on and gracefuly report.
	_, err := user.Current()
	if err != nil {
		log.Fatalf("mc: Unable to obtain user's home directory. \nERROR[%v]\n", err)
	}

	// Ensures config file is sane and cached to __config private variable.
	_ = getMcConfig()
}

func main() {
	app := cli.NewApp()
	app.Usage = "Minio Client for S3 Compatible Object Storage"
	app.Version = "0.1.0"
	app.Commands = options
	app.Flags = flags
	app.Author = "Minio.io"
	app.EnableBashCompletion = true
	app.Run(os.Args)
}
