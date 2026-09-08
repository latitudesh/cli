// Package s3test is an in-memory, path-style S3 server for unit tests. It
// implements the subset of the API the CLI uses (ListObjectsV2, ListObjectVersions,
// Put/Get/Head/Delete object, multi-delete, multipart upload, server-side copy)
// and records every request so tests can assert on headers (no checksum
// trailers, signing region, path style) without a real backend.
package s3test

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Object is a stored object.
type Object struct {
	Key          string
	Data         []byte
	ContentType  string
	Metadata     map[string]string // x-amz-meta-* (lower-case keys without prefix)
	Headers      map[string]string // other response headers (Cache-Control…)
	LastModified time.Time
	ETag         string
	VersionID    string
}

// Bucket is a stored bucket.
type Bucket struct {
	Name    string
	Objects map[string]*Object
	// Versions keeps non-current versions when Versioned is true.
	Versions  map[string][]*Object
	Versioned bool
}

// Request is a recorded incoming request.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   []byte
}

// Server is the fake S3 endpoint.
type Server struct {
	mu       sync.Mutex
	buckets  map[string]*Bucket
	requests []Request
	uploads  map[string]*multipart
	httptest *httptest.Server
	// DenyAll makes every request fail with AccessDenied (403).
	DenyAll bool
	// FailNext, when set, is returned once for the next request.
	FailNext *ErrorResponse
	// SlowListPages forces ListObjectsV2 to paginate with at most N keys per
	// page when max-keys is larger, to exercise continuation tokens.
	MaxListKeys int
	now         func() time.Time
}

type multipart struct {
	bucket, key string
	parts       map[int][]byte
	contentType string
	metadata    map[string]string
	headers     map[string]string
}

// ErrorResponse is an S3 XML error.
type ErrorResponse struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource,omitempty"`
	RequestID string   `xml:"RequestId"`
	Region    string   `xml:"Region,omitempty"`
	Status    int      `xml:"-"`
}

// New starts a server. Call Close when done.
func New() *Server {
	s := &Server{
		buckets: map[string]*Bucket{},
		uploads: map[string]*multipart{},
		now:     time.Now,
	}
	s.httptest = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// URL returns the endpoint (http://127.0.0.1:port).
func (s *Server) URL() string { return s.httptest.URL }

// Close shuts the server down.
func (s *Server) Close() { s.httptest.Close() }

// CreateBucket adds an empty bucket.
func (s *Server) CreateBucket(name string) *Bucket {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := &Bucket{Name: name, Objects: map[string]*Object{}, Versions: map[string][]*Object{}}
	s.buckets[name] = b
	return b
}

// AddObject stores an object (creating the bucket if needed).
func (s *Server) AddObject(bucket, key string, data []byte, contentType string) *Object {
	s.mu.Lock()
	b := s.buckets[bucket]
	if b == nil {
		b = &Bucket{Name: bucket, Objects: map[string]*Object{}, Versions: map[string][]*Object{}}
		s.buckets[bucket] = b
	}
	s.mu.Unlock()
	return s.put(b, key, data, contentType, nil, nil)
}

// Object returns a stored object or nil.
func (s *Server) Object(bucket, key string) *Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.buckets[bucket]
	if b == nil {
		return nil
	}
	return b.Objects[key]
}

// Keys returns the sorted object keys of a bucket.
func (s *Server) Keys(bucket string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.buckets[bucket]
	if b == nil {
		return nil
	}
	keys := make([]string, 0, len(b.Objects))
	for k := range b.Objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Requests returns a copy of the recorded requests.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.requests))
	copy(out, s.requests)
	return out
}

// WriteRequests returns the recorded requests that mutate state.
func (s *Server) WriteRequests() []Request {
	var out []Request
	for _, r := range s.Requests() {
		if r.Method == http.MethodPut || r.Method == http.MethodPost || r.Method == http.MethodDelete {
			out = append(out, r)
		}
	}
	return out
}

// ResetRequests clears the request log.
func (s *Server) ResetRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = nil
}

func etagOf(data []byte) string {
	sum := md5.Sum(data)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func (s *Server) put(b *Bucket, key string, data []byte, contentType string, meta, headers map[string]string) *Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	if contentType == "" {
		contentType = "binary/octet-stream"
	}
	o := &Object{Key: key, Data: data, ContentType: contentType, Metadata: meta, Headers: headers, LastModified: s.now().UTC().Truncate(time.Second), ETag: etagOf(data)}
	if b.Versioned {
		if prev := b.Objects[key]; prev != nil {
			b.Versions[key] = append(b.Versions[key], prev)
		}
		o.VersionID = fmt.Sprintf("v%d", len(b.Versions[key])+1)
	}
	b.Objects[key] = o
	return o
}

func (s *Server) record(r *http.Request, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Header: r.Header.Clone(), Body: body})
}

func writeError(w http.ResponseWriter, e ErrorResponse) {
	if e.RequestID == "" {
		e.RequestID = "s3test"
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(e.Status)
	_ = xml.NewEncoder(w).Encode(e)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if isAWSChunked(r.Header) {
		body = decodeAWSChunked(body)
	}
	s.record(r, body)

	s.mu.Lock()
	failNext := s.FailNext
	s.FailNext = nil
	deny := s.DenyAll
	s.mu.Unlock()
	if failNext != nil {
		writeError(w, *failNext)
		return
	}
	if deny {
		writeError(w, ErrorResponse{Code: "AccessDenied", Message: "Access Denied", Status: 403})
		return
	}

	// Path-style: /{bucket}[/{key}]
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		s.listBuckets(w)
		return
	}
	bucketName, key := path, ""
	if i := strings.Index(path, "/"); i >= 0 {
		bucketName, key = path[:i], path[i+1:]
	}
	key, _ = url.PathUnescape(key)

	s.mu.Lock()
	b := s.buckets[bucketName]
	s.mu.Unlock()
	if b == nil {
		writeError(w, ErrorResponse{Code: "NoSuchBucket", Message: "The specified bucket does not exist", Resource: "/" + bucketName, Status: 404})
		return
	}
	q := r.URL.Query()

	if key == "" {
		switch {
		case r.Method == http.MethodGet && q.Has("versions"):
			s.listVersions(w, b, q)
		case r.Method == http.MethodGet && q.Get("list-type") == "2":
			s.listV2(w, b, q)
		case r.Method == http.MethodGet && q.Has("location"):
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<LocationConstraint>us-east-1</LocationConstraint>`)
		case r.Method == http.MethodGet:
			// ListObjects v1 fallback: treat as v2 without tokens.
			s.listV2(w, b, q)
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && q.Has("delete"):
			s.multiDelete(w, b, body)
		default:
			writeError(w, ErrorResponse{Code: "MethodNotAllowed", Message: "method not allowed on bucket", Status: 405})
		}
		return
	}

	switch r.Method {
	case http.MethodPut:
		if src := r.Header.Get("X-Amz-Copy-Source"); src != "" {
			s.copyObject(w, b, key, src, r.Header)
			return
		}
		if q.Has("uploadId") {
			s.uploadPart(w, q.Get("uploadId"), q.Get("partNumber"), body)
			return
		}
		meta, headers := extractMeta(r.Header)
		o := s.put(b, key, body, r.Header.Get("Content-Type"), meta, headers)
		w.Header().Set("ETag", o.ETag)
		if o.VersionID != "" {
			w.Header().Set("x-amz-version-id", o.VersionID)
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodPost:
		switch {
		case q.Has("uploads"):
			s.initiateMultipart(w, b, key, r.Header)
		case q.Has("uploadId"):
			s.completeMultipart(w, b, key, q.Get("uploadId"), body)
		default:
			writeError(w, ErrorResponse{Code: "MethodNotAllowed", Message: "bad POST", Status: 405})
		}
	case http.MethodGet, http.MethodHead:
		s.getObject(w, r, b, key, q.Get("versionId"))
	case http.MethodDelete:
		if q.Has("uploadId") {
			s.mu.Lock()
			delete(s.uploads, q.Get("uploadId"))
			s.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		s.mu.Lock()
		if vid := q.Get("versionId"); vid != "" {
			s.deleteVersion(b, key, vid)
		} else {
			delete(b.Objects, key)
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, ErrorResponse{Code: "MethodNotAllowed", Message: "method not allowed", Status: 405})
	}
}

func (s *Server) deleteVersion(b *Bucket, key, vid string) {
	if cur := b.Objects[key]; cur != nil && cur.VersionID == vid {
		vs := b.Versions[key]
		if len(vs) > 0 {
			b.Objects[key] = vs[len(vs)-1]
			b.Versions[key] = vs[:len(vs)-1]
		} else {
			delete(b.Objects, key)
		}
		return
	}
	vs := b.Versions[key]
	for i, v := range vs {
		if v.VersionID == vid {
			b.Versions[key] = append(vs[:i], vs[i+1:]...)
			return
		}
	}
}

func extractMeta(h http.Header) (map[string]string, map[string]string) {
	meta := map[string]string{}
	headers := map[string]string{}
	for k, v := range h {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-amz-meta-") && len(v) > 0 {
			meta[strings.TrimPrefix(lk, "x-amz-meta-")] = v[0]
		}
		switch lk {
		case "cache-control", "content-encoding", "content-disposition", "content-language", "expires":
			if len(v) > 0 {
				headers[http.CanonicalHeaderKey(lk)] = v[0]
			}
		}
	}
	return meta, headers
}

type listBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Buckets []struct {
		Name         string `xml:"Name"`
		CreationDate string `xml:"CreationDate"`
	} `xml:"Buckets>Bucket"`
}

func (s *Server) listBuckets(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var res listBucketsResult
	for name := range s.buckets {
		res.Buckets = append(res.Buckets, struct {
			Name         string `xml:"Name"`
			CreationDate string `xml:"CreationDate"`
		}{name, s.now().UTC().Format(time.RFC3339)})
	}
	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(res)
}

type listV2Result struct {
	XMLName               xml.Name         `xml:"ListBucketResult"`
	Name                  string           `xml:"Name"`
	Prefix                string           `xml:"Prefix"`
	Delimiter             string           `xml:"Delimiter,omitempty"`
	KeyCount              int              `xml:"KeyCount"`
	MaxKeys               int              `xml:"MaxKeys"`
	IsTruncated           bool             `xml:"IsTruncated"`
	ContinuationToken     string           `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string           `xml:"NextContinuationToken,omitempty"`
	Contents              []contentsEntry  `xml:"Contents"`
	CommonPrefixes        []commonPrefixes `xml:"CommonPrefixes"`
}

type contentsEntry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type commonPrefixes struct {
	Prefix string `xml:"Prefix"`
}

func (s *Server) listV2(w http.ResponseWriter, b *Bucket, q url.Values) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := q.Get("prefix")
	delimiter := q.Get("delimiter")
	maxKeys := 1000
	if mk := q.Get("max-keys"); mk != "" {
		if n, err := strconv.Atoi(mk); err == nil && n > 0 {
			maxKeys = n
		}
	}
	if s.MaxListKeys > 0 && maxKeys > s.MaxListKeys {
		maxKeys = s.MaxListKeys
	}
	startAfter := q.Get("start-after")
	if tok := q.Get("continuation-token"); tok != "" {
		startAfter = tok
	}

	keys := make([]string, 0, len(b.Objects))
	for k := range b.Objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	res := listV2Result{Name: b.Name, Prefix: prefix, Delimiter: delimiter, MaxKeys: maxKeys, ContinuationToken: q.Get("continuation-token")}
	seenPrefix := map[string]bool{}
	count := 0
	var last string
	for _, k := range keys {
		if startAfter != "" && k <= startAfter {
			continue
		}
		if count >= maxKeys {
			res.IsTruncated = true
			res.NextContinuationToken = last
			break
		}
		if delimiter != "" {
			rest := k[len(prefix):]
			if i := strings.Index(rest, delimiter); i >= 0 {
				cp := prefix + rest[:i+len(delimiter)]
				if !seenPrefix[cp] {
					seenPrefix[cp] = true
					res.CommonPrefixes = append(res.CommonPrefixes, commonPrefixes{Prefix: cp})
					count++
				}
				last = cp + "￿" // skip everything under this prefix
				continue
			}
		}
		o := b.Objects[k]
		res.Contents = append(res.Contents, contentsEntry{Key: k, LastModified: o.LastModified.Format("2006-01-02T15:04:05.000Z"), ETag: o.ETag, Size: int64(len(o.Data)), StorageClass: "STANDARD"})
		count++
		last = k
	}
	if res.IsTruncated && strings.HasSuffix(res.NextContinuationToken, "￿") {
		// Keys sorted after a prefix marker: use the marker's prefix as token.
		res.NextContinuationToken = strings.TrimSuffix(res.NextContinuationToken, "￿") + "￿"
	}
	res.KeyCount = count
	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(res)
}

type listVersionsResult struct {
	XMLName        xml.Name         `xml:"ListVersionsResult"`
	Name           string           `xml:"Name"`
	Prefix         string           `xml:"Prefix"`
	IsTruncated    bool             `xml:"IsTruncated"`
	Versions       []versionEntry   `xml:"Version"`
	DeleteMarkers  []versionEntry   `xml:"DeleteMarker"`
	CommonPrefixes []commonPrefixes `xml:"CommonPrefixes"`
}

type versionEntry struct {
	Key          string `xml:"Key"`
	VersionID    string `xml:"VersionId"`
	IsLatest     bool   `xml:"IsLatest"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
}

func (s *Server) listVersions(w http.ResponseWriter, b *Bucket, q url.Values) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := q.Get("prefix")
	res := listVersionsResult{Name: b.Name, Prefix: prefix}
	keys := make([]string, 0, len(b.Objects))
	for k := range b.Objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		o := b.Objects[k]
		vid := o.VersionID
		if vid == "" {
			vid = "null"
		}
		res.Versions = append(res.Versions, versionEntry{Key: k, VersionID: vid, IsLatest: true, LastModified: o.LastModified.Format("2006-01-02T15:04:05.000Z"), ETag: o.ETag, Size: int64(len(o.Data))})
		for i := len(b.Versions[k]) - 1; i >= 0; i-- {
			v := b.Versions[k][i]
			res.Versions = append(res.Versions, versionEntry{Key: k, VersionID: v.VersionID, LastModified: v.LastModified.Format("2006-01-02T15:04:05.000Z"), ETag: v.ETag, Size: int64(len(v.Data))})
		}
	}
	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(res)
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request, b *Bucket, key, versionID string) {
	s.mu.Lock()
	o := b.Objects[key]
	if o != nil && versionID != "" && o.VersionID != versionID {
		o = nil
		for _, v := range b.Versions[key] {
			if v.VersionID == versionID {
				o = v
			}
		}
	}
	s.mu.Unlock()
	if o == nil {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeError(w, ErrorResponse{Code: "NoSuchKey", Message: "The specified key does not exist.", Resource: "/" + b.Name + "/" + key, Status: 404})
		return
	}
	h := w.Header()
	h.Set("Content-Type", o.ContentType)
	h.Set("ETag", o.ETag)
	h.Set("Last-Modified", o.LastModified.UTC().Format(http.TimeFormat))
	h.Set("Accept-Ranges", "bytes")
	if o.VersionID != "" {
		h.Set("x-amz-version-id", o.VersionID)
	}
	for k, v := range o.Metadata {
		h.Set("X-Amz-Meta-"+k, v)
	}
	for k, v := range o.Headers {
		h.Set(k, v)
	}
	data := o.Data
	status := http.StatusOK
	if rng := r.Header.Get("Range"); rng != "" && strings.HasPrefix(rng, "bytes=") {
		parts := strings.SplitN(strings.TrimPrefix(rng, "bytes="), "-", 2)
		start, _ := strconv.Atoi(parts[0])
		end := len(data) - 1
		if len(parts) == 2 && parts[1] != "" {
			if e, err := strconv.Atoi(parts[1]); err == nil && e < end {
				end = e
			}
		}
		if start > end || start >= len(data) {
			writeError(w, ErrorResponse{Code: "InvalidRange", Message: "The requested range is not satisfiable", Status: 416})
			return
		}
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		data = data[start : end+1]
		status = http.StatusPartialContent
	}
	h.Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	if r.Method == http.MethodGet {
		_, _ = w.Write(data)
	}
}

type deleteRequest struct {
	XMLName xml.Name `xml:"Delete"`
	Quiet   bool     `xml:"Quiet"`
	Objects []struct {
		Key       string `xml:"Key"`
		VersionID string `xml:"VersionId"`
	} `xml:"Object"`
}

type deleteResult struct {
	XMLName xml.Name `xml:"DeleteResult"`
	Deleted []struct {
		Key       string `xml:"Key"`
		VersionID string `xml:"VersionId,omitempty"`
	} `xml:"Deleted"`
	Errors []struct {
		Key     string `xml:"Key"`
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	} `xml:"Error"`
}

func (s *Server) multiDelete(w http.ResponseWriter, b *Bucket, body []byte) {
	var req deleteRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeError(w, ErrorResponse{Code: "MalformedXML", Message: err.Error(), Status: 400})
		return
	}
	var res deleteResult
	s.mu.Lock()
	for _, o := range req.Objects {
		if o.VersionID != "" {
			s.deleteVersion(b, o.Key, o.VersionID)
		} else {
			delete(b.Objects, o.Key)
		}
		if !req.Quiet {
			res.Deleted = append(res.Deleted, struct {
				Key       string `xml:"Key"`
				VersionID string `xml:"VersionId,omitempty"`
			}{o.Key, o.VersionID})
		}
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	_ = xml.NewEncoder(w).Encode(res)
}

func (s *Server) copyObject(w http.ResponseWriter, b *Bucket, key, src string, h http.Header) {
	src = strings.TrimPrefix(src, "/")
	src, _ = url.PathUnescape(src)
	srcBucket, srcKey := src, ""
	if i := strings.Index(src, "/"); i >= 0 {
		srcBucket, srcKey = src[:i], src[i+1:]
	}
	if i := strings.Index(srcKey, "?versionId="); i >= 0 {
		srcKey = srcKey[:i]
	}
	s.mu.Lock()
	sb := s.buckets[srcBucket]
	var so *Object
	if sb != nil {
		so = sb.Objects[srcKey]
	}
	s.mu.Unlock()
	if so == nil {
		writeError(w, ErrorResponse{Code: "NoSuchKey", Message: "The specified key does not exist.", Status: 404})
		return
	}
	meta, headers := so.Metadata, so.Headers
	contentType := so.ContentType
	if strings.EqualFold(h.Get("X-Amz-Metadata-Directive"), "REPLACE") {
		meta, headers = extractMeta(h)
		if ct := h.Get("Content-Type"); ct != "" {
			contentType = ct
		}
	}
	o := s.put(b, key, append([]byte(nil), so.Data...), contentType, meta, headers)
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<CopyObjectResult><LastModified>%s</LastModified><ETag>%s</ETag></CopyObjectResult>`, o.LastModified.Format(time.RFC3339), o.ETag)
}

func (s *Server) initiateMultipart(w http.ResponseWriter, b *Bucket, key string, h http.Header) {
	s.mu.Lock()
	id := fmt.Sprintf("upload-%d", len(s.uploads)+1)
	meta, headers := extractMeta(h)
	s.uploads[id] = &multipart{bucket: b.Name, key: key, parts: map[int][]byte{}, contentType: h.Get("Content-Type"), metadata: meta, headers: headers}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<InitiateMultipartUploadResult><Bucket>%s</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>`, b.Name, xmlEscape(key), id)
}

func (s *Server) uploadPart(w http.ResponseWriter, uploadID, partNumber string, body []byte) {
	n, err := strconv.Atoi(partNumber)
	s.mu.Lock()
	up := s.uploads[uploadID]
	s.mu.Unlock()
	if err != nil || up == nil {
		writeError(w, ErrorResponse{Code: "NoSuchUpload", Message: "The specified upload does not exist.", Status: 404})
		return
	}
	s.mu.Lock()
	up.parts[n] = append([]byte(nil), body...)
	s.mu.Unlock()
	w.Header().Set("ETag", etagOf(body))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) completeMultipart(w http.ResponseWriter, b *Bucket, key, uploadID string, body []byte) {
	s.mu.Lock()
	up := s.uploads[uploadID]
	if up != nil {
		delete(s.uploads, uploadID)
	}
	s.mu.Unlock()
	if up == nil {
		writeError(w, ErrorResponse{Code: "NoSuchUpload", Message: "The specified upload does not exist.", Status: 404})
		return
	}
	nums := make([]int, 0, len(up.parts))
	for n := range up.parts {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var data []byte
	for _, n := range nums {
		data = append(data, up.parts[n]...)
	}
	o := s.put(b, key, data, up.contentType, up.metadata, up.headers)
	o.ETag = fmt.Sprintf(`"%s-%d"`, strings.Trim(etagOf(data), `"`), len(nums))
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<CompleteMultipartUploadResult><Bucket>%s</Bucket><Key>%s</Key><ETag>%s</ETag></CompleteMultipartUploadResult>`, b.Name, xmlEscape(key), o.ETag)
}

func xmlEscape(s string) string {
	var sb strings.Builder
	_ = xml.EscapeText(&sb, []byte(s))
	return sb.String()
}

// HasChecksumHeaders reports whether any recorded write request carried
// x-amz-checksum-* / x-amz-sdk-checksum-algorithm headers or declared
// checksum trailers — what S3-compatible backends such as Wasabi reject.
// (Plain SigV4 streaming over http is not a checksum trailer.)
func (s *Server) HasChecksumHeaders() bool {
	for _, r := range s.WriteRequests() {
		for k := range r.Header {
			lk := strings.ToLower(k)
			if strings.HasPrefix(lk, "x-amz-checksum-") || lk == "x-amz-sdk-checksum-algorithm" || lk == "x-amz-trailer" {
				return true
			}
		}
		if strings.Contains(strings.ToUpper(r.Header.Get("X-Amz-Content-Sha256")), "TRAILER") {
			return true
		}
	}
	return false
}

// SigningRegionOf extracts the region from a request's SigV4 Authorization
// header ("" when absent).
func SigningRegionOf(r Request) string {
	auth := r.Header.Get("Authorization")
	i := strings.Index(auth, "Credential=")
	if i < 0 {
		return ""
	}
	parts := strings.Split(auth[i+len("Credential="):], "/")
	if len(parts) < 3 {
		return ""
	}
	return parts[2]
}

// isAWSChunked reports whether the request body uses the SigV4 streaming
// framing (`aws-chunked`), which minio-go emits over plain http.
func isAWSChunked(h http.Header) bool {
	if strings.Contains(strings.ToLower(h.Get("Content-Encoding")), "aws-chunked") {
		return true
	}
	return strings.HasPrefix(strings.ToUpper(h.Get("X-Amz-Content-Sha256")), "STREAMING-")
}

// decodeAWSChunked strips the `<hex-size>;chunk-signature=…\r\n<data>\r\n`
// framing (and any trailing headers after the terminating 0 chunk).
func decodeAWSChunked(body []byte) []byte {
	var out []byte
	rest := body
	for {
		nl := strings.Index(string(rest), "\r\n")
		if nl < 0 {
			return out
		}
		header := string(rest[:nl])
		rest = rest[nl+2:]
		sizeHex := header
		if i := strings.IndexByte(header, ';'); i >= 0 {
			sizeHex = header[:i]
		}
		size, err := strconv.ParseInt(strings.TrimSpace(sizeHex), 16, 64)
		if err != nil {
			// Not framed after all: return the original body.
			return body
		}
		if size == 0 {
			return out
		}
		if int64(len(rest)) < size {
			return out
		}
		out = append(out, rest[:size]...)
		rest = rest[size:]
		if len(rest) >= 2 && rest[0] == '\r' && rest[1] == '\n' {
			rest = rest[2:]
		}
	}
}
