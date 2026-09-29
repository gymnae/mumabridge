package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	serverName string
	asToken    string
	http       *http.Client
}

func NewClient(baseURL, serverName, asToken string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), serverName: serverName, asToken: asToken,
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) UserID(localpart string) string { return "@" + localpart + ":" + c.serverName }

func (c *Client) EnsureGhost(ctx context.Context, localpart, displayName, roomID string) error {
	register := map[string]any{"type": "m.login.application_service", "username": localpart}
	if err := c.request(ctx, http.MethodPost, "/_matrix/client/v3/register", "", register); err != nil && !isMatrixError(err, "M_USER_IN_USE") {
		return fmt.Errorf("register ghost: %w", err)
	}
	userID := c.UserID(localpart)
	profilePath := "/_matrix/client/v3/profile/" + url.PathEscape(userID) + "/displayname"
	if err := c.request(ctx, http.MethodPut, profilePath, userID, map[string]string{"displayname": displayName}); err != nil {
		return fmt.Errorf("set ghost display name: %w", err)
	}
	joinPath := "/_matrix/client/v3/rooms/" + url.PathEscape(roomID) + "/join"
	if err := c.request(ctx, http.MethodPost, joinPath, userID, map[string]any{}); err != nil {
		return fmt.Errorf("join ghost to room: %w", err)
	}
	return nil
}

func (c *Client) LeaveGhost(ctx context.Context, userID, roomID string) error {
	path := "/_matrix/client/v3/rooms/" + url.PathEscape(roomID) + "/leave"
	return c.request(ctx, http.MethodPost, path, userID, map[string]any{})
}

type matrixError struct {
	Status  int    `json:"-"`
	ErrCode string `json:"errcode"`
	Message string `json:"error"`
}

func (e matrixError) Error() string {
	return fmt.Sprintf("Matrix HTTP %d %s: %s", e.Status, e.ErrCode, e.Message)
}

func isMatrixError(err error, code string) bool {
	matrixErr, ok := err.(matrixError)
	return ok && matrixErr.ErrCode == code
}

func (c *Client) request(ctx context.Context, method, path, userID string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	query := endpoint.Query()
	if userID != "" {
		query.Set("user_id", userID)
	}
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.asToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	var matrixErr matrixError
	matrixErr.Status = resp.StatusCode
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&matrixErr); err != nil {
		matrixErr.Message = resp.Status
	}
	return matrixErr
}
