// 앱 ↔ 서버 공유 타입. spec §2, §3.

// 설정의 주인은 앱 로컬(AsyncStorage). DB는 푸시용 사본. (spec §2)
export interface Settings {
  home: LatLng;
  work: LatLng;
  commuteStart: string; // "0900" (HHmm, KST)
  commuteEnd: string; // "1800"
  notificationsEnabled: boolean;
}

export interface LatLng {
  lat: number;
  lng: number;
}

// GET /forecast 응답의 카드 한 장 (출근 또는 퇴근). spec §3.
export interface SlotForecast {
  skyText: string; // 맑음/구름많음/흐림
  ptyText: string; // 없음/비/눈/소나기
  tempC: number;
  popPct: number; // 강수확률
  needUmbrella: boolean;
  feelsVsYesterday?: string; // "어제보다 추워요" (고도화)
  hourly?: HourlyPoint[]; // 점진적 공개용 시간별 흐름 (spec §7.1)
}

export interface HourlyPoint {
  time: string; // "0800"
  tempC: number;
  popPct: number;
  ptyText: string;
}

export interface ForecastResponse {
  morning: SlotForecast;
  evening: SlotForecast;
}
