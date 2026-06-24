package push

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSendSerialization 은 요청 바디 JSON 직렬화 형태와 헤더를 mock 서버로 검증한다.
// 실제 Expo 호출은 하지 않는다.
func TestSendSerialization(t *testing.T) {
	var gotBody map[string]any
	var gotContentType, gotAccept string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"status":"ok"}}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.Send(context.Background(), Message{
		To:    "ExponentPushToken[abc]",
		Title: "우산챙겨?",
		Body:  "오늘 출근길 비 소식, 우산 챙기세요",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q; want application/json", gotContentType)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q; want application/json", gotAccept)
	}
	if gotBody["to"] != "ExponentPushToken[abc]" {
		t.Errorf("body.to = %v; want ExponentPushToken[abc]", gotBody["to"])
	}
	if gotBody["title"] != "우산챙겨?" {
		t.Errorf("body.title = %v", gotBody["title"])
	}
	if gotBody["body"] != "오늘 출근길 비 소식, 우산 챙기세요" {
		t.Errorf("body.body = %v", gotBody["body"])
	}
	if gotBody["sound"] != "default" {
		t.Errorf("body.sound = %v; want default", gotBody["sound"])
	}
}

// TestSendExpoError 는 status != ok 응답을 에러로 처리하는지 본다.
func TestSendExpoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"status":"error","message":"DeviceNotRegistered"}}`))
	}))
	defer srv.Close()

	err := New(srv.URL).Send(context.Background(), Message{To: "ExponentPushToken[x]"})
	if err == nil {
		t.Fatal("expected error for status=error, got nil")
	}
	if !strings.Contains(err.Error(), "DeviceNotRegistered") {
		t.Errorf("error missing expo message: %v", err)
	}
}

// TestSendHTTPError 는 비-200 응답을 에러로 처리하는지 본다.
func TestSendHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	err := New(srv.URL).Send(context.Background(), Message{To: "ExponentPushToken[x]"})
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error missing status code: %v", err)
	}
}
