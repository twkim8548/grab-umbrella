// 서버 API 클라이언트. spec §3. 동기화는 항상 로컬 → DB 단방향.
import type { ForecastResponse, Settings } from "./types";

// TODO: 환경별 분기 (Render 배포 URL). app.config 또는 expo-constants 로 주입.
const BASE_URL = "http://localhost:8080";

// POST /sync — 설정이 바뀔 때마다 호출. 위경도 그대로 올리면 서버가 격자 변환.
export async function sync(pushToken: string, s: Settings): Promise<void> {
  const res = await fetch(`${BASE_URL}/sync`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      push_token: pushToken,
      home_lat: s.home.lat,
      home_lng: s.home.lng,
      work_lat: s.work.lat,
      work_lng: s.work.lng,
      commute_start: s.commuteStart,
      commute_end: s.commuteEnd,
    }),
  });
  if (!res.ok) throw new Error(`sync failed: ${res.status}`);
}

// GET /forecast — 메인 화면용 출근/퇴근 카드 데이터.
export async function getForecast(pushToken: string): Promise<ForecastResponse> {
  const res = await fetch(`${BASE_URL}/forecast?push_token=${encodeURIComponent(pushToken)}`);
  if (!res.ok) throw new Error(`forecast failed: ${res.status}`);
  return (await res.json()) as ForecastResponse;
}
