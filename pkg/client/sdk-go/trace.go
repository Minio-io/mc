/*
 * Minimalist Object Storage SDK (C) 2015 Minio, Inc.
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

package minio

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"

	"github.com/minio-io/mc/pkg/console"
	"github.com/minio-io/minio/pkg/iodine"
)

// Trace - tracing structure
type Trace struct {
	BodyTraceFlag        bool      // Include Body
	RequestTransportFlag bool      // Include additional http.Transport adds such as User-Agent
	Writer               io.Writer // Console device to write
}

// NewTrace - initialize Trace structure
func NewTrace(bodyTraceFlag, requestTransportFlag bool, writer io.Writer) HTTPTracer {
	t := Trace{
		BodyTraceFlag:        bodyTraceFlag,
		RequestTransportFlag: requestTransportFlag,
		Writer:               writer,
	}
	return t
}

// Request - Trace HTTP Request
func (t Trace) Request(req *http.Request) (err error) {
	origAuthKey := req.Header.Get("Authorization")
	req.Header.Set("Authorization", "AWS **PASSWORD**STRIPPED**")

	if t.RequestTransportFlag {
		reqTrace, err := httputil.DumpRequestOut(req, t.BodyTraceFlag)
		if err == nil {
			t.print(reqTrace)
		}
	} else {
		reqTrace, err := httputil.DumpRequest(req, t.BodyTraceFlag)
		if err == nil {
			t.print(reqTrace)
		}
	}

	req.Header.Set("Authorization", origAuthKey)
	return iodine.New(err, nil)
}

// Response - Trace HTTP Response
func (t Trace) Response(res *http.Response) (err error) {
	resTrace, err := httputil.DumpResponse(res, t.BodyTraceFlag)
	if err == nil {
		t.print(resTrace)
	}
	return iodine.New(err, nil)
}

// print HTTP Response
func (t Trace) print(data []byte) {
	if t.Writer != nil {
		fmt.Fprintf(t.Writer, "%s", data)
	} else {
		console.Debugf("%s", data)
	}
}
