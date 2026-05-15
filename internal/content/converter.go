package content

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ContentConverter interface {
	ConvertToMarkdown(ctx context.Context, url string) (string, error)
}

type JinaConverter struct {
	timeout time.Duration
	client  *http.Client
}

const JinaBaseURL = "https://r.jina.ai"

func NewJinaConverter(timeout time.Duration) *JinaConverter {
	return &JinaConverter{
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (j *JinaConverter) ConvertToMarkdown(ctx context.Context, url string) (res string, err error) {
	jinaURL := JinaBaseURL + "/" + url

	req, err := http.NewRequestWithContext(ctx, "GET", jinaURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "text/markdown")

	resp, err := j.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(resp.Body.Close(), err) }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("jina conversion failed: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), err
}
