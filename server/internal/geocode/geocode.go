// Package geocode 는 카카오 로컬 API로 주소를 위경도로 변환한다. spec §2.
// 주소→위경도 변환은 서버 한 곳에서만 한다(앱은 도로명 주소 문자열만 보낸다).
package geocode

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

const kakaoAddressURL = "https://dapi.kakao.com/v2/local/search/address.json"

// Client 는 카카오 로컬 API 호출을 담당한다.
type Client struct {
	restKey    string
	httpClient *http.Client
}

func New(restKey string) *Client {
	return &Client{
		restKey:    restKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// kakaoAddressResponse 는 address.json 응답 구조다. documents[0] 의 x=경도, y=위도(문자열).
type kakaoAddressResponse struct {
	Documents []struct {
		AddressName string `json:"address_name"`
		X           string `json:"x"` // 경도 (lng)
		Y           string `json:"y"` // 위도 (lat)
	} `json:"documents"`
}

// Geocode 는 주소를 위경도로 변환한다. documents[0] 의 y→lat, x→lng.
func (c *Client) Geocode(ctx context.Context, address string) (lat, lng float64, err error) {
	if c.restKey == "" {
		return 0, 0, fmt.Errorf("kakao key not configured")
	}

	u := kakaoAddressURL + "?query=" + url.QueryEscape(address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: build request: %w", err)
	}
	req.Header.Set("Authorization", "KakaoAK "+c.restKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("geocode: kakao status %d: %s", resp.StatusCode, string(body))
	}

	return parseGeocode(body)
}

// parseGeocode 는 응답 JSON 바이트를 lat/lng 로 파싱한다(순수 함수, 테스트용 분리).
func parseGeocode(body []byte) (lat, lng float64, err error) {
	var r kakaoAddressResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return 0, 0, fmt.Errorf("geocode: parse response: %w", err)
	}
	if len(r.Documents) == 0 {
		return 0, 0, fmt.Errorf("geocode: 주소를 찾을 수 없습니다")
	}
	doc := r.Documents[0]
	lat, err = strconv.ParseFloat(doc.Y, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: parse lat %q: %w", doc.Y, err)
	}
	lng, err = strconv.ParseFloat(doc.X, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode: parse lng %q: %w", doc.X, err)
	}
	return lat, lng, nil
}
