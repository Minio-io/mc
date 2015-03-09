// Original license //
// ---------------- //

/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// All other modifications and improvements //
// ---------------------------------------- //

/*
 * Mini Object Storage, (C) 2015 Minio, Inc.
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

// Package s3 implements a generic Amazon S3 client
package s3

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Total max object list
const (
	MaxKeys = 1000
)

// Client is an Amazon S3 client.
type Client struct {
	*Auth
	Transport http.RoundTripper // or nil for the default
}

// Bucket - carries s3 bucket reply header
type Bucket struct {
	Name         string
	CreationDate xmlTime
}

func (c *Client) transport() http.RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	return http.DefaultTransport
}

// bucketURL returns the URL prefix of the bucket, with trailing slash
func (c *Client) bucketURL(bucket string) string {
	var url string
	if IsValidBucket(bucket) && !strings.Contains(bucket, ".") {
		// if localhost forcePathStyle
		if strings.Contains(c.endpoint(), "localhost") || strings.Contains(c.endpoint(), "127.0.0.1") {
			url = fmt.Sprintf("%s/%s", c.endpoint(), bucket)
			goto ret
		}
		host, _, _ := net.SplitHostPort(c.Endpoint)
		if net.ParseIP(host) != nil {
			url = fmt.Sprintf("%s/%s", c.endpoint(), bucket)
			goto ret
		}
		if !c.S3ForcePathStyle {
			if strings.Contains(c.endpoint(), "amazonaws.com") {
				url = fmt.Sprintf("https://%s.%s/", bucket, strings.TrimPrefix(c.endpoint(), "https://"))
			} else {
				url = fmt.Sprintf("http://%s.%s/", bucket, strings.TrimPrefix(c.endpoint(), "http://"))
			}
		} else {
			url = fmt.Sprintf("%s/%s", c.endpoint(), bucket)
		}
	}

ret:
	return url
}

func (c *Client) keyURL(bucket, key string) string {
	// if localhost forcePathStyle
	host, _, _ := net.SplitHostPort(c.Endpoint)
	ok := (strings.Contains(c.endpoint(), "localhost") || strings.Contains(bucket, "127.0.0.1") || c.S3ForcePathStyle || net.ParseIP(host) != nil)
	if ok {
		return c.bucketURL(bucket) + "/" + key
	}
	return c.bucketURL(bucket) + key
}

func newReq(url string) *http.Request {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(fmt.Sprintf("s3 client; invalid URL: %v", err))
	}
	req.Header.Set("User-Agent", "Minio Client")
	return req
}

func parseListAllMyBuckets(r io.Reader) ([]*Bucket, error) {
	type allMyBuckets struct {
		Buckets struct {
			Bucket []*Bucket
		}
	}
	var res allMyBuckets
	if err := xml.NewDecoder(r).Decode(&res); err != nil {
		return nil, err
	}
	return res.Buckets.Bucket, nil
}

/// Object API operations

// Buckets - Get list of buckets
func (c *Client) Buckets() ([]*Bucket, error) {
	req := newReq(c.endpoint() + "/")
	c.Auth.signRequest(req)
	res, err := c.transport().RoundTrip(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("s3: Unexpected status code %d fetching bucket list", res.StatusCode)
	}
	return parseListAllMyBuckets(res.Body)
}

// Stat - returns 0, "", os.ErrNotExist if not on S3
func (c *Client) Stat(key, bucket string) (size int64, date time.Time, reterr error) {
	req := newReq(c.keyURL(bucket, key))
	req.Method = "HEAD"
	c.Auth.signRequest(req)
	res, err := c.transport().RoundTrip(req)
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return 0, date, err
	}

	switch res.StatusCode {
	case http.StatusNotFound:
		return 0, date, os.ErrNotExist
	case http.StatusOK:
		size, err = strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
		if err != nil {
			return 0, date, err
		}
		if dateStr := res.Header.Get("Last-Modified"); dateStr != "" {
			// AWS S3 uses RFC1123 standard for Date in HTTP header, unlike XML content
			date, err := time.Parse(time.RFC1123, dateStr)
			if err != nil {
				return 0, date, err
			}
			return size, date, nil
		}
	default:
		return 0, date, fmt.Errorf("s3: Unexpected status code %d statting object %v", res.StatusCode, key)
	}
	return
}

// PutBucket - create new bucket
func (c *Client) PutBucket(bucket string) error {
	var url string
	if IsValidBucket(bucket) && !strings.Contains(bucket, ".") {
		url = fmt.Sprintf("%s/%s", c.endpoint(), bucket)
	}
	req := newReq(url)
	req.Method = "PUT"
	c.Auth.signRequest(req)
	res, err := c.transport().RoundTrip(req)
	if err != nil {
		return err
	}

	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Got response code %d from s3", res.StatusCode)
	}
	return nil

}

// Put - upload new object to bucket
func (c *Client) Put(bucket, key string, size int64, contents io.Reader) error {
	req := newReq(c.keyURL(bucket, key))
	req.Method = "PUT"
	req.ContentLength = size

	h := md5.New()
	// Memory where data is present
	sink := new(bytes.Buffer)
	mw := io.MultiWriter(h, sink)
	written, err := io.Copy(mw, contents)
	if written != size {
		return fmt.Errorf("Data read mismatch")
	}
	if err != nil {
		return err
	}
	req.Body = ioutil.NopCloser(sink)
	b64 := base64.StdEncoding.EncodeToString(h.Sum(nil))
	req.Header.Set("Content-MD5", b64)
	c.Auth.signRequest(req)

	res, err := c.transport().RoundTrip(req)
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}

	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		// res.Write(os.Stderr)
		return fmt.Errorf("Got response code %d from s3", res.StatusCode)
	}
	return nil
}

// Item - object item list
type Item struct {
	Key          string
	LastModified xmlTime
	Size         int64
}

// Prefix - common prefix
type Prefix struct {
	Prefix string
}

// BySize implements sort.Interface for []Item based on
// the Size field.
type BySize []*Item

func (a BySize) Len() int           { return len(a) }
func (a BySize) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BySize) Less(i, j int) bool { return a[i].Size < a[j].Size }

type listBucketResults struct {
	Contents       []*Item
	IsTruncated    bool
	MaxKeys        int
	Name           string // bucket name
	Marker         string
	Delimiter      string
	Prefix         string
	CommonPrefixes []*Prefix
}

// BucketLocation - returns the S3 endpoint to be used with the given bucket.
func (c *Client) BucketLocation(bucket string) (location string, err error) {
	if !strings.HasSuffix(c.endpoint(), "amazonaws.com") {
		return "", errors.New("BucketLocation not implemented for non-Amazon S3 endpoints")
	}
	urlReq := fmt.Sprintf("%s/%s/?location", c.endpoint(), url.QueryEscape(bucket))
	req := newReq(urlReq)
	c.Auth.signRequest(req)
	res, err := c.transport().RoundTrip(req)
	if err != nil {
		return
	}
	var xres xmlLocationConstraint
	if err := xml.NewDecoder(res.Body).Decode(&xres); err != nil {
		return "", err
	}
	if xres.Location == "" {
		return strings.TrimPrefix(c.endpoint(), "https://"), nil
	}
	return "s3-" + xres.Location + ".amazonaws.com", nil
}

// GetBucket (List Objects) returns 0 to maxKeys (inclusive) items from the
// provided bucket. Keys before startAt will be skipped. (This is the S3
// 'marker' value). If the length of the returned items is equal to
// maxKeys, there is no indication whether or not the returned list is truncated.
func (c *Client) GetBucket(bucket string, startAt, prefix, delimiter string, maxKeys int) (items []*Item, prefixes []*Prefix, err error) {
	var urlReq string
	var buffer bytes.Buffer

	if maxKeys <= 0 {
		return nil, nil, errors.New("negative maxKeys are invalid")
	}

	marker := startAt
	for len(items) < maxKeys {
		fetchN := maxKeys - len(items)
		if fetchN > MaxKeys {
			fetchN = MaxKeys
		}
		var bres listBucketResults
		buffer.WriteString(fmt.Sprintf("%s?max-keys=%d", c.bucketURL(bucket), fetchN))
		switch true {
		case marker != "":
			buffer.WriteString(fmt.Sprintf("&marker=%s", url.QueryEscape(marker)))
			fallthrough
		case prefix != "":
			buffer.WriteString(fmt.Sprintf("&prefix=%s", url.QueryEscape(prefix)))
			fallthrough
		case delimiter != "":
			buffer.WriteString(fmt.Sprintf("&delimiter=%s", url.QueryEscape(delimiter)))
		}

		urlReq = buffer.String()
		// Try the enumerate three times, since Amazon likes to close
		// https connections a lot, and Go sucks at dealing with it:
		// https://code.google.com/p/go/issues/detail?id=3514
		const maxTries = 5
		for try := 1; try <= maxTries; try++ {
			time.Sleep(time.Duration(try-1) * 100 * time.Millisecond)
			req := newReq(urlReq)
			c.Auth.signRequest(req)
			res, err := c.transport().RoundTrip(req)
			if err != nil {
				if try < maxTries {
					continue
				}
				return nil, nil, err
			}
			if res.StatusCode != http.StatusOK {
				if res.StatusCode < 500 {
					body, _ := ioutil.ReadAll(io.LimitReader(res.Body, 1<<20))
					aerr := &Error{
						Op:     "ListBucket",
						Code:   res.StatusCode,
						Body:   body,
						Header: res.Header,
					}
					aerr.parseXML()
					res.Body.Close()
					return nil, nil, aerr
				}
			} else {
				bres = listBucketResults{}
				var logbuf bytes.Buffer
				err = xml.NewDecoder(io.TeeReader(res.Body, &logbuf)).Decode(&bres)
				if err != nil {
					log.Printf("Error parsing s3 XML response: %v for %q", err, logbuf.Bytes())
				} else if bres.MaxKeys != fetchN || bres.Name != bucket || bres.Marker != marker {
					err = fmt.Errorf("Unexpected parse from server: %#v from: %s", bres, logbuf.Bytes())
					log.Print(err)
				}
			}
			res.Body.Close()
			if err != nil {
				if try < maxTries-1 {
					continue
				}
				log.Print(err)
				return nil, nil, err
			}
			break
		}
		for _, it := range bres.Contents {
			if it.Key == marker && it.Key != startAt {
				// Skip first dup on pages 2 and higher.
				continue
			}
			if it.Key < startAt {
				return nil, nil, fmt.Errorf("Unexpected response from Amazon: item key %q but wanted greater than %q", it.Key, startAt)
			}
			items = append(items, it)
			marker = it.Key
		}

		for _, pre := range bres.CommonPrefixes {
			if pre.Prefix != "" {
				prefixes = append(prefixes, pre)
			}
		}

		if !bres.IsTruncated {
			break
		}

		if len(items) == 0 {
			return nil, nil, errors.New("No items replied")
		}
	}
	return items, prefixes, nil
}

// Get - download a requested object from a given bucket
func (c *Client) Get(bucket, key string) (body io.ReadCloser, size int64, err error) {
	req := newReq(c.keyURL(bucket, key))
	c.Auth.signRequest(req)
	res, err := c.transport().RoundTrip(req)
	if err != nil {
		return
	}
	switch res.StatusCode {
	case http.StatusOK:
		return res.Body, res.ContentLength, nil
	case http.StatusNotFound:
		res.Body.Close()
		return nil, 0, os.ErrNotExist
	default:
		res.Body.Close()
		return nil, 0, fmt.Errorf("Amazon HTTP error on GET: %d", res.StatusCode)
	}
}

// GetPartial fetches part of the s3 key object in bucket.
// If length is negative, the rest of the object is returned.
// The caller must close rc.
func (c *Client) GetPartial(bucket, key string, offset, length int64) (rc io.ReadCloser, err error) {
	if offset < 0 {
		return nil, errors.New("invalid negative length")
	}

	req := newReq(c.keyURL(bucket, key))
	if length >= 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
	} else {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	c.Auth.signRequest(req)

	res, err := c.transport().RoundTrip(req)
	if err != nil {
		return
	}
	switch res.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		return res.Body, nil
	case http.StatusNotFound:
		res.Body.Close()
		return nil, os.ErrNotExist
	default:
		res.Body.Close()
		return nil, fmt.Errorf("Amazon HTTP error on GET: %d", res.StatusCode)
	}
}

/* Not supporting Delete's
func (c *Client) Delete(bucket, key string) error {
	req := newReq(c.keyURL(bucket, key))
	req.Method = "DELETE"
	c.Auth.signRequest(req)
	res, err := c.transport().RoundTrip(req)
	if err != nil {
		return err
	}
	if res != nil && res.Body != nil {
		defer res.Body.Close()
	}
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusNoContent ||
		res.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("Amazon HTTP error on DELETE: %d", res.StatusCode)
}
*/

// NewClient - get new client
func NewClient(auth *Auth) (client *Client) {
	client = &Client{auth, http.DefaultTransport}
	return
}

// IsValidBucket reports whether bucket is a valid bucket name, per Amazon's naming restrictions.
// See http://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html
func IsValidBucket(bucket string) bool {
	if len(bucket) < 3 || len(bucket) > 63 {
		return false
	}
	if bucket[0] == '.' || bucket[len(bucket)-1] == '.' {
		return false
	}
	if match, _ := regexp.MatchString("\\.\\.", bucket); match == true {
		return false
	}
	// We don't support buckets with '.' in them
	match, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9\\-]+[a-zA-Z0-9]$", bucket)
	return match
}
