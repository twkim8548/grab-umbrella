// Package push 는 Expo Push 발송을 담당한다. spec §5.
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 는 Expo Push API 호출을 담당한다.
type Client struct {
	url        string
	httpClient *http.Client
}

func New(url string) *Client {
	return &Client{
		url:        url,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Message 는 한 줄 알림. spec §5: 제목 "우산챙겨?" / 본문 "오늘 퇴근길 비 와요".
type Message struct {
	To    string // Expo 푸시 토큰
	Title string
	Body  string
}

// expoRequest 는 Expo Push API 단일 발송 바디다.
// Expo 는 단일 객체/배열 모두 받지만 단건이므로 단일 객체로 보낸다.
type expoRequest struct {
	To    string `json:"to"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Sound string `json:"sound"`
}

// expoResponse 는 Expo Push API 응답이다. { "data": { "status": "ok"|"error", "message": ... } }.
type expoResponse struct {
	Data struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"data"`
}

// Send 는 Expo Push API 로 발송한다. 순수 발송만 담당하며 토큰 유효성(dev/정식)은
// 호출부(cron)에서 거른다. status != "ok" 또는 비-200 응답이면 에러를 반환한다.
func (c *Client) Send(ctx context.Context, m Message) error {
	payload, err := json.Marshal(expoRequest{
		To:    m.To,
		Title: m.Title,
		Body:  m.Body,
		Sound: "default",
	})
	if err != nil {
		return fmt.Errorf("push: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("push: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("push: http do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("push: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("push: http status %d: %s", resp.StatusCode, string(body))
	}

	var r expoResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("push: parse json: %w", err)
	}
	if r.Data.Status != "ok" {
		return fmt.Errorf("push: expo status %q: %s", r.Data.Status, r.Data.Message)
	}
	return nil
}
