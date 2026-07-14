// 서버 API 클라이언트. spec §3. 동기화는 항상 로컬 → DB 단방향.
import Constants from "expo-constants";
import type { ForecastResponse, Settings } from "./types";

// 실기기는 Mac 의 localhost 에 접근할 수 없으므로 LAN IP 로 서버를 가리켜야 한다.
// app.json 의 extra.apiBaseUrl 로 주입(없으면 LAN IP 기본값). 시뮬레이터/실기기 공통.
// 네트워크가 바뀌면(다른 와이파이) app.json 의 IP 만 갱신하면 된다.
const BASE_URL: string =
  Constants.expoConfig?.extra?.apiBaseUrl ?? "http://192.168.0.79:8080";

// POST /sync — 설정이 바뀔 때마다 호출. 주소를 올리면 서버가 지오코딩/격자 변환.
export async function sync(
  pushToken: string,
  s: Settings,
  signal?: AbortSignal
): Promise<void> {
  const res = await fetch(`${BASE_URL}/sync`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    signal,
    body: JSON.stringify({
      push_token: pushToken,
      home_address: s.homeAddress,
      work_address: s.workAddress,
      commute_start: s.commuteStart,
      commute_end: s.commuteEnd,
      commute_days: s.commuteDays,
      notifications_enabled: s.notificationsEnabled,
    }),
  });
  if (res.ok) return;

  // 상태코드별 에러 메시지. 422(주소 못찾음)는 본문 텍스트를 그대로 노출.
  if (res.status === 400) throw new Error("필수 항목이 누락되었습니다.");
  if (res.status === 422) {
    const text = (await res.text().catch(() => "")).trim();
    throw new Error(text || "주소를 찾을 수 없습니다.");
  }
  if (res.status >= 500) throw new Error("서버 오류가 발생했습니다. 잠시 후 다시 시도해주세요.");
  throw new Error(`동기화 실패 (${res.status})`);
}

// forecast 가 404(서버에 미등록)일 때 던지는 식별용 메시지.
// 신규기기/미동기화는 "에러"가 아니라 "설정 필요" 상태이므로 호출부가 구분한다.
export const NOT_REGISTERED = "NOT_REGISTERED";

// 개발용 시나리오 프리뷰: app.json extra.mockForecast 에 값(예 "rain,sunny,shower-later,cloudy")
// 이 있으면 /forecast?mock=... 으로 가짜 날씨를 받아 UI 를 확인한다. 비우면 실제 API.
const MOCK_FORECAST: string | undefined = Constants.expoConfig?.extra?.mockForecast;

// GET /forecast — 메인 화면용 출근/퇴근 카드 데이터. 슬롯은 nullable.
// 404 는 "서버에 미등록"(신규기기/미동기화)을 의미하므로 NOT_REGISTERED 로 구분해 던진다.
export async function getForecast(
  pushToken: string,
  signal?: AbortSignal
): Promise<ForecastResponse> {
  const query = MOCK_FORECAST
    ? `mock=${encodeURIComponent(MOCK_FORECAST)}`
    : `push_token=${encodeURIComponent(pushToken)}`;
  const res = await fetch(`${BASE_URL}/forecast?${query}`, { signal });
  if (res.status === 404) throw new Error(NOT_REGISTERED);
  if (!res.ok) throw new Error(`forecast failed: ${res.status}`);
  return (await res.json()) as ForecastResponse;
}
