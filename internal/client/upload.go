// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// uploadURL derives the HTTP upload endpoint (/_upload) from the WebSocket
// endpoint: same host, wss:// -> https://, ws:// -> http://.
func uploadURL(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint %q: %w", endpoint, err)
	}
	scheme := "https"
	if u.Scheme == "ws" || u.Scheme == "http" {
		scheme = "http"
	}
	return (&url.URL{Scheme: scheme, Host: u.Host, Path: "/_upload"}).String(), nil
}

// PutFile writes the content read from content to path on the TrueNAS host
// with filesystem.put and waits for its job. filesystem.put takes the file
// through an input pipe, which the JSON-RPC WebSocket can't carry, so the
// content goes over HTTP to /_upload, authorized by a short-lived,
// single-use token this session generates (auth.generate_token). The token
// works for both API key and password sessions, and never leaves this
// request. The content is streamed, never held in memory as a whole, so
// large files work. mode is the file's Unix mode (e.g. 0o644).
func (c *Client) PutFile(ctx context.Context, path string, content io.Reader, mode int) error {
	target, err := uploadURL(c.endpoint)
	if err != nil {
		return err
	}

	raw, err := c.Call(ctx, "auth.generate_token", 300, map[string]any{}, true, true)
	if err != nil {
		return fmt.Errorf("auth.generate_token: %w", err)
	}
	var token string
	if err := json.Unmarshal(raw, &token); err != nil || token == "" {
		return fmt.Errorf("auth.generate_token: unexpected result %s", raw)
	}
	meta, err := json.Marshal(map[string]any{
		"method": "filesystem.put",
		"params": []any{path, map[string]any{"mode": mode}},
	})
	if err != nil {
		return err
	}

	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	go func() {
		pw.CloseWithError(writeUpload(w, meta, content))
	}()
	defer func() { _ = pr.Close() }()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, pr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("User-Agent", c.userAgent)

	httpClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: c.tlsConfig, Proxy: http.ProxyFromEnvironment},
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s returned %d: %s", target, resp.StatusCode, rb)
	}
	var jr struct {
		JobID int64 `json:"job_id"`
	}
	if err := json.Unmarshal(rb, &jr); err != nil || jr.JobID == 0 {
		return fmt.Errorf("POST %s: unexpected response %q", target, rb)
	}
	return c.waitJob(ctx, jr.JobID)
}

// writeUpload writes /_upload's multipart body: the "data" field with the
// method call, then the "file" part with the content.
func writeUpload(w *multipart.Writer, meta []byte, content io.Reader) error {
	if err := w.WriteField("data", string(meta)); err != nil {
		return err
	}
	fw, err := w.CreateFormFile("file", "upload")
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, content); err != nil {
		return err
	}
	return w.Close()
}

// waitJob polls core.get_jobs until the job jobID ends, and returns an error
// unless it ended in SUCCESS. Unlike CallJob, the id is known to be a job's.
func (c *Client) waitJob(ctx context.Context, jobID int64) error {
	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			jobs, err := c.Call(ctx, "core.get_jobs", []any{[]any{"id", "=", jobID}})
			if err != nil {
				return fmt.Errorf("polling job %d: %w", jobID, err)
			}
			var entries []jobEntry
			if err := json.Unmarshal(jobs, &entries); err != nil {
				return fmt.Errorf("polling job %d: %w", jobID, err)
			}
			if len(entries) == 0 {
				continue
			}
			switch entries[0].State {
			case "SUCCESS":
				return nil
			case "FAILED", "ABORTED":
				return fmt.Errorf("job %d %s: %s", jobID, entries[0].State, entries[0].Error)
			}
		}
	}
}
