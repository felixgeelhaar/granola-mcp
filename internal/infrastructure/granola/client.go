package granola

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client wraps the Granola REST API.
// This is an infrastructure concern — the domain has no knowledge of HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

func NewClient(baseURL string, httpClient *http.Client, token string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		token:      token,
	}
}

func (c *Client) SetToken(token string) {
	c.token = token
}

func (c *Client) GetDocuments(ctx context.Context, since *time.Time, limit, offset int) (*DocumentListResponse, error) {
	params := url.Values{}
	if since != nil {
		params.Set("since", since.Format(time.RFC3339))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	var resp DocumentListResponse
	if err := c.get(ctx, "/v2/get-documents", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetDocument(ctx context.Context, id string) (*DocumentDTO, error) {
	params := url.Values{}
	params.Set("id", id)

	var resp DocumentDTO
	if err := c.get(ctx, "/v2/get-document", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetTranscript(ctx context.Context, meetingID string) (*TranscriptResponse, error) {
	params := url.Values{}
	params.Set("meeting_id", meetingID)

	var resp TranscriptResponse
	if err := c.get(ctx, "/v2/get-document-transcript", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetWorkspaces(ctx context.Context) (*WorkspaceListResponse, error) {
	var resp WorkspaceListResponse
	if err := c.get(ctx, "/v2/get-workspaces", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, target interface{}) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimited
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}
