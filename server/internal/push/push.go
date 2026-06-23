// Package push 는 Expo Push 발송을 담당한다. spec §5.
package push

import "context"

type Client struct {
	url string
}

func New(url string) *Client { return &Client{url: url} }

// Message 는 한 줄 알림. spec §5: 제목 "우산챙겨?" / 본문 "오늘 퇴근길 비 와요".
type Message struct {
	To    string // Expo 푸시 토큰
	Title string
	Body  string
}

// Send 는 Expo Push API 로 발송한다.
func (c *Client) Send(ctx context.Context, m Message) error {
	// TODO: POST {url} with JSON {to, title, body, sound} ; 응답/에러 처리
	return errNotImplemented
}

type sentinelError string

func (e sentinelError) Error() string { return string(e) }

const errNotImplemented = sentinelError("push: not implemented yet")
