// 앱 ↔ 서버 공유 타입. spec §2, §3.

// 설정의 주인은 앱 로컬(AsyncStorage). DB는 푸시용 사본. (spec §2)
// 주소 기반: 좌표 변환은 서버(/sync)가 담당한다.
export interface Settings {
  homeAddress: string; // 도로명 주소(지오코딩/서버 전송용)
  workAddress: string;
  homeDong: string; // 동네 표시용 (예: "역삼동" 또는 "수지구"). 카드에 표시.
  workDong: string;
  commuteStart: string; // "0830" (HHmm, KST)
  commuteEnd: string; // "1900"
  // 출근일(요일) 7자리 "0111110" — 일~토 순서, 1=on. 이 요일에만 푸시 발송.
  // 기본 평일(월~금)="0111110". 미설정(구버전 로컬값) 시 평일로 폴백.
  commuteDays: string;
  notificationsEnabled: boolean;
}

// GET /forecast 응답의 카드 한 장 (출근 또는 퇴근). spec §3.
export interface SlotForecast {
  skyText: string; // 맑음/구름많음/흐림
  ptyText: string; // 없음/비/눈/소나기
  tempC: number;
  popPct: number; // 강수확률
  needUmbrella: boolean;
  // 대표값(anchor)과 우산 결론이 어긋날 때의 근거. 예: 출근 정시는 맑지만
  // 윈도우 안 "19시부터 소나기"라 우산이 필요한 경우. 일치하면 빈 문자열.
  umbrellaReason: string;
  feelsVsYesterday?: string; // "어제보다 추워요" (고도화)
  hourly?: HourlyPoint[]; // 점진적 공개용 시간별 흐름 (spec §7.1)
}

export interface HourlyPoint {
  time: string; // "0800"
  tempC: number;
  popPct: number;
  ptyText: string;
}

// 하루(오늘/내일) 단위 출퇴근 두 시점. 이미 지난 시점은 null. (서버 계약)
export interface DayForecast {
  morning: SlotForecast | null;
  evening: SlotForecast | null;
}

// GET /forecast 응답: 오늘·내일 각각의 출퇴근 4시점. 지난 시점은 null.
export interface ForecastResponse {
  today: DayForecast;
  tomorrow: DayForecast;
}
