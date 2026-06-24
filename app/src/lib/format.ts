// 공통 포맷 헬퍼. 시각 "0830" → "8:30".
// MainScreen/CommuteCard/HourlySheet 에서 동일하게 사용.
export function formatHHmm(hhmm: string): string {
  if (hhmm.length !== 4) return hhmm;
  return `${Number(hhmm.slice(0, 2))}:${hhmm.slice(2)}`;
}
