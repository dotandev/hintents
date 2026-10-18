// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package daemon

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipContentEncoding is the token a client advertises in Accept-Encoding and
// the value we set on compressed responses.
const gzipContentEncoding = "gzip"

// gzipWriterPool reuses gzip.Writer instances across requests so we do not
// re-allocate the deflate state for every response. The writer is always
// re-pointed at the target ResponseWriter with Reset before use.
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

// acceptsGzip reports whether the request advertises gzip support in its
// Accept-Encoding header. The header is a comma-separated list where each
// entry may carry parameters (for example "br;q=0.8, gzip;q=1.0"), so the
// parameters are stripped before the comparison.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		encoding := strings.TrimSpace(part)
		if i := strings.IndexByte(encoding, ';'); i >= 0 {
			encoding = strings.TrimSpace(encoding[:i])
		}
		if strings.EqualFold(encoding, gzipContentEncoding) {
			return true
		}
	}
	return false
}

// gzipResponseWriter transparently compresses every byte written through it.
//
// If the wrapped handler emits a response that must not carry a body (for
// example 204 No Content or 304 Not Modified) compression is disabled and the
// response is passed through unchanged.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	compress    bool
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	if statusCode < http.StatusOK || statusCode == http.StatusNoContent || statusCode == http.StatusNotModified {
		// These responses must not carry a body, so do not advertise or
		// emit a compressed stream.
		w.compress = false
		w.Header().Del("Content-Encoding")
		w.Header().Del("Vary")
	}

	// The compressed length differs from whatever the handler intended, so
	// drop Content-Length and let net/http fall back to chunked encoding.
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if !w.compress {
		return w.ResponseWriter.Write(b)
	}
	return w.gz.Write(b)
}

// gzipHandler wraps h so that responses are gzip-compressed when, and only
// when, the client advertises gzip support via Accept-Encoding. Clients that
// do not send the header receive the uncompressed response unchanged. A
// Vary: Accept-Encoding header is added so shared caches keep the two
// representations apart.
func gzipHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HEAD responses carry no body, so compression buys nothing.
		if !acceptsGzip(r) || r.Method == http.MethodHead {
			h.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", gzipContentEncoding)
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")

		gz := gzipWriterPool.Get().(*gzip.Writer)
		gz.Reset(w)

		gw := &gzipResponseWriter{ResponseWriter: w, gz: gz, compress: true}
		h.ServeHTTP(gw, r)

		if gw.compress && gw.wroteHeader {
			// Close emits the gzip trailer; for an empty body this still
			// produces a valid, decodable empty gzip stream.
			_ = gz.Close()
		}
		// Detach the writer from the response before returning it to the
		// pool so no buffered bytes leak into the next response.
		gz.Reset(io.Discard)
		gzipWriterPool.Put(gz)
	})
}
