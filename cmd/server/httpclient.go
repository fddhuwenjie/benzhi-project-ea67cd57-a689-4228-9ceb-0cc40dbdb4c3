package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type smokeClient struct {
	base string
	http *http.Client
	seq  int
}

func (c *smokeClient) id() string { c.seq++; return fmt.Sprintf("selftest-%02d", c.seq) }
func (c *smokeClient) request(method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(b)
	}
	req, e := http.NewRequest(method, c.base+path, reader)
	if e != nil {
		return e
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := c.http.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%s %s: HTTP %d: %s", method, path, res.StatusCode, b)
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}
func newSmokeClient(base string) *smokeClient {
	return &smokeClient{base: base, http: &http.Client{Timeout: 4 * time.Second}}
}
