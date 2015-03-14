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
	"fmt"
	"io"
	"os"

	"github.com/cheggaaa/pb"
	"github.com/codegangsta/cli"
	"github.com/minio-io/mc/pkg/s3"
)

// Different modes of cp operation
const (
	first  = iota // <Object> <S3Object> or <Object> <S3Bucket>
	second        // <S3Object> <Object> or <S3Object> .
	third         // <S3Object> <S3Object> or <S3Object> <S3Bucket>
	fourth        // <Dir> <S3Bucket> or <S3Bucket> <Dir> or <Dir> <S3Uri>
	invalid
)

// Get current mode of operation from available arguments and options
func getMode(recursive bool, args *cmdArgs) int {
	switch recursive {
	case false:
		switch true {
		// <Object> <S3Object> or <Object> <S3Bucket>
		case args.source.bucket == "" && args.destination.bucket != "":
			return first
		// <S3Object> <Object> or <S3Object> .
		case args.source.bucket != "" && args.source.key != "" && args.destination.bucket == "":
			return second
		// <S3Object> <S3Object> or <S3Object> <S3Bucket>
		case args.source.bucket != "" && args.destination.bucket != "" && args.source.key != "":
			return third
		}
	case true:
		switch true {
		// <Dir> <S3Bucket> or <S3Bucket> <Dir> or <Dir> <S3Uri>
		case args.source.bucket != "" || args.source.key != "":
			return fourth
		}
	}
	return invalid
}

// First mode <Object> <S3Object> or <Object> <S3Bucket>
func firstMode(s3c *s3.Client, args *cmdArgs) error {
	if args.source.key == "" {
		return fmt.Errorf("invalid args")
	}
	st, err := os.Stat(args.source.key)
	if os.IsNotExist(err) {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("omitting directory '%s'", st.Name())
	}
	size := st.Size()
	source, err := os.Open(args.source.key)
	defer source.Close()
	if err != nil {
		return err
	}

	// http://<bucket>.<hostname> is specified without key
	if args.destination.key == "" {
		args.destination.key = args.source.key
	}

	err = s3c.Put(args.destination.bucket, args.destination.key, size, source)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("%s uploaded -- to bucket:(http://%s/%s)", args.source.key,
		args.destination.bucket, args.destination.key)
	info(msg)
	return nil
}

// Second mode <S3Object> <Object> or <S3Object> .
func secondMode(s3c *s3.Client, args *cmdArgs) error {
	var objectReader io.ReadCloser
	var objectSize, downloadedSize int64
	var destination *os.File
	var err error
	var st os.FileInfo

	// Send HEAD request to validate if file exists.
	objectSize, _, err = s3c.Stat(args.source.bucket, args.source.key)
	if err != nil {
		return err
	}

	var bar *pb.ProgressBar
	if !args.quiet {
		// get progress bar
		bar = startBar(objectSize)
	}

	// Check if the object already exists
	st, err = os.Stat(args.destination.key)
	switch os.IsNotExist(err) {
	case true:
		// Create if it doesn't exist
		destination, err = os.Create(args.destination.key)
		defer destination.Close()
		if err != nil {
			return err
		}
		objectReader, _, err = s3c.Get(args.source.bucket, args.source.key)
		if err != nil {
			return err
		}
	case false:
		downloadedSize = st.Size()
		// Verify if file is already downloaded
		if downloadedSize == objectSize {
			return fmt.Errorf("%s object has been already downloaded", args.destination.key)
		}

		destination, err = os.OpenFile(args.destination.key, os.O_RDWR, 0600)
		defer destination.Close()
		if err != nil {
			return err
		}

		_, err := destination.Seek(downloadedSize, os.SEEK_SET)
		if err != nil {
			return err
		}

		remainingSize := objectSize - downloadedSize
		objectReader, objectSize, err = s3c.GetPartial(args.source.bucket,
			args.source.key, downloadedSize, remainingSize)
		if err != nil {
			return err
		}

		if !args.quiet {
			bar.Set(int(downloadedSize))
		}
	}

	writer := io.Writer(destination)
	if !args.quiet {
		// Start the bar now
		bar.Start()
		// create multi writer to feed data
		writer = io.MultiWriter(destination, bar)
	}

	_, err = io.CopyN(writer, objectReader, objectSize)
	if err != nil {
		return err
	}

	bar.Finish()
	info("Success!")
	return nil
}

// <S3Object> <S3Object> or <S3Object> <S3Bucket>
func thirdMode(s3c *s3.Client, args *cmdArgs) error {
	var objectReader io.ReadCloser
	var objectSize int64
	var err error

	// Send HEAD request to validate if file exists.
	objectSize, _, err = s3c.Stat(args.source.bucket, args.source.key)
	if err != nil {
		return err
	}

	if args.destination.key == "" {
		args.destination.key = args.source.key
	}

	// Check if the object already exists
	_, _, err = s3c.Stat(args.destination.bucket, args.destination.key)
	switch os.IsNotExist(err) {
	case true:
		objectReader, _, err = s3c.Get(args.source.bucket, args.source.key)
		if err != nil {
			return err
		}
		err = s3c.Put(args.destination.bucket, args.destination.key, objectSize, objectReader)
		if err != nil {
			return err
		}
	case false:
		return fmt.Errorf("Ranges not supported")
	}

	msg := fmt.Sprintf("http://%s/%s uploaded -- to bucket:(http://%s/%s)", args.source.bucket, args.source.key,
		args.destination.bucket, args.destination.key)
	info(msg)
	return nil
}

func fourthMode(s3c *s3.Client, args *cmdArgs) error {
	if args.source.bucket == "" {
		_, err := os.Stat(args.source.key)
		if os.IsNotExist(err) {
			return err
		}
		if args.destination.bucket == "" {
			args.destination.bucket = args.source.key
		}
	} else {
		if args.destination.key == "" {
			args.destination.key = args.source.bucket
		}
		_, err := os.Stat(args.destination.key)
		if os.IsNotExist(err) {
			os.MkdirAll(args.destination.key, 0755)
		}
	}
	return doRecursiveCp(s3c, args)
}

func doFsCopy(c *cli.Context) {
	s3c, err := getNewClient(c)
	if err != nil {
		fatal(err.Error())
	}

	if len(c.Args()) != 2 {
		fatal("Invalid number of args")
	}

	var cmdargs *cmdArgs
	cmdargs, err = parseArgs(c)
	if err != nil {
		fatal(err.Error())
	}

	switch getMode(c.Bool("recursive"), cmdargs) {
	case first:
		err := firstMode(s3c, cmdargs)
		if err != nil {
			fatal(err.Error())
		}
	case second:
		err := secondMode(s3c, cmdargs)
		if err != nil {
			fatal(err.Error())
		}
	case third:
		err := thirdMode(s3c, cmdargs)
		if err != nil {
			fatal(err.Error())
		}
	case fourth:
		err := fourthMode(s3c, cmdargs)
		if err != nil {
			fatal(err.Error())
		}
	default:
		fatal("invalid args")
	}
}
