package fetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const UserAgent = "logpose (+https://github.com/VTGare/logpose)"

// RecheckAfter is how old a page that still changes may be before it's read again.
const RecheckAfter = 20 * time.Hour

// NewClient is the HTTP client card data and art are fetched with.
func NewClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// LoadJSON reads a cache file into v. found is false when there's no file yet, and when the
// file doesn't parse, since a cache holds pages that can be read again. v may be half filled
// then, so the caller starts from a new value.
func LoadJSON(path string, v any) (found bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	err = json.Unmarshal(data, v)
	if errors.As(err, &syntax) || errors.As(err, &typ) {
		return false, nil
	}
	return err == nil, err
}

// SaveJSON writes v to path through a temporary file, so a crash never leaves half a cache.
func SaveJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Get reads url's body. A body over limit bytes is an error, so a page is never read in part.
func Get(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%s: over %d bytes", url, limit)
	}
	return body, nil
}

func Wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
