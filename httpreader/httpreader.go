package httpreader

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// var traceCtx = httptrace.WithClientTrace(context.Background(), &httptrace.ClientTrace{
// 	GotConn: func(info httptrace.GotConnInfo) {
// 		log.Printf("conn was reused: %t", info.Reused)
// 	},
// })

const kCacheSize = 1024 * 1024 * 4

type HTTPReader struct {
	Client      *http.Client
	URL         string
	TotalLength int
	Cache       [kCacheSize]byte
	CacheOffset int64
}

func (h *HTTPReader) ReadAt(p []byte, off int64) (int, error) {

	if off >= h.CacheOffset && off+int64(len(p)) < h.CacheOffset+kCacheSize {
		var subslice = h.Cache[(off - h.CacheOffset):]
		return copy(p, subslice[:len(p)]), nil
	} else if len(p) > kCacheSize {

		var req, _ = http.NewRequest(http.MethodGet, h.URL, nil)

		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(len(p)-1)))

		var rsp, err = h.Client.Do(req)
		if err != nil {
			return 0, err
		}
		defer rsp.Body.Close()

		if rsp.StatusCode != 206 {
			return 0, fmt.Errorf("bad status code: %d", rsp.StatusCode)
		}

		return io.ReadFull(rsp.Body, p)

	} else {

		var req, _ = http.NewRequest(http.MethodGet, h.URL, nil)

		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+int64(kCacheSize-1)))

		var rsp, err = h.Client.Do(req)
		if err != nil {
			return 0, err
		}
		defer rsp.Body.Close()

		if rsp.StatusCode != 206 {
			return 0, fmt.Errorf("bad status code: %d", rsp.StatusCode)
		}

		h.CacheOffset = off
		io.ReadFull(rsp.Body, h.Cache[:])

		return copy(p, h.Cache[:len(p)]), nil
	}

}

func CreateHTTPReader(url string, startOffset int, maxLength int) (*io.SectionReader, error) {

	var client = &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: 2,
		},
	}

	var rsp, err = client.Head(url)
	if err != nil {
		return nil, err
	}
	rsp.Body.Close()

	if rsp.Header.Get("Accept-Ranges") != "bytes" {
		return nil, fmt.Errorf("server does not accept range requests")
	}

	contentLen, err := strconv.Atoi(rsp.Header.Get("Content-Length"))
	if err != nil {
		return nil, err
	}

	var limit = contentLen

	if maxLength != -1 {
		limit = maxLength
	}

	return io.NewSectionReader(&HTTPReader{
		Client:      client,
		URL:         url,
		TotalLength: contentLen,
		CacheOffset: int64(contentLen),
	}, int64(startOffset), int64(limit)), nil

}
