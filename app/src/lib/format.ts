// 공통 포맷 헬퍼. 시각 "0830" → "8:30".
// MainScreen/CommuteCard/HourlySheet 에서 동일하게 사용.
export function formatHHmm(hhmm: string): string {
  if (hhmm.length !== 4) return hhmm;
  return `${Number(hhmm.slice(0, 2))}:${hhmm.slice(2)}`;
}

// 출퇴근 시각("0830")이 오늘인지 내일인지 판단해 라벨을 반환한다.
// 서버 SlotDateTime 과 동일 규칙: 오늘 그 시각을 이미 지났으면 내일, 아니면 오늘.
// (예: 17시에 출근 7:30 은 지났으므로 "내일", 퇴근 18:00 은 아직이라 "오늘".)
export function dayLabel(hhmm: string, now: Date = new Date()): "오늘" | "내일" {
  if (hhmm.length !== 4) return "오늘";
  const hh = Number(hhmm.slice(0, 2));
  const mm = Number(hhmm.slice(2));
  const slot = new Date(now);
  slot.setHours(hh, mm, 0, 0);
  return now.getTime() > slot.getTime() ? "내일" : "오늘";
}
