import { View, Text, Pressable, StyleSheet } from "react-native";
import type { SlotForecast } from "../lib/types";

// 출근/퇴근 카드 (보조 정보). spec §7.1: 우산 여부·기온·강수확률을 압축.
// 메인 상단 결론이 주인공이므로 카드는 절제된 크기로.
// 탭하면 hourly(시간별 흐름)를 인라인으로 펼친다. hourly 없으면 탭 비활성.
export default function CommuteCard({
  label,
  time,
  data,
  expanded,
  onToggle,
}: {
  label: string; // "출근" | "퇴근"
  time: string; // "8:30" 표시용
  data: SlotForecast | null;
  expanded: boolean;
  onToggle: () => void;
}) {
  const hasHourly = !!data?.hourly && data.hourly.length > 0;

  return (
    <Pressable
      style={styles.card}
      onPress={hasHourly ? onToggle : undefined}
      disabled={!hasHourly}
    >
      <View style={styles.headerRow}>
        <Text style={styles.label}>{label}</Text>
        <Text style={styles.time}>{time}</Text>
      </View>

      {data ? (
        <>
          <Text style={styles.umbrella}>{data.needUmbrella ? "☔️" : "🌤"}</Text>
          <Text style={styles.temp}>{Math.round(data.tempC)}°</Text>
          <Text style={styles.meta}>
            {data.skyText} · {data.popPct}%
          </Text>

          {hasHourly && expanded ? (
            <View style={styles.hourlyBox}>
              {data.hourly!.map((h) => (
                <View key={h.time} style={styles.hourlyRow}>
                  <Text style={styles.hourlyTime}>{formatHHmm(h.time)}</Text>
                  <Text style={styles.hourlyTemp}>{Math.round(h.tempC)}°</Text>
                  <Text style={styles.hourlyPop}>{h.popPct}%</Text>
                  <Text style={styles.hourlyPty}>{h.ptyText}</Text>
                </View>
              ))}
            </View>
          ) : null}

          {hasHourly ? (
            <Text style={styles.hint}>{expanded ? "접기" : "시간별 보기"}</Text>
          ) : null}
        </>
      ) : (
        <Text style={styles.meta}>정보 없음</Text>
      )}
    </Pressable>
  );
}

// "0830" → "8:30"
function formatHHmm(hhmm: string): string {
  if (hhmm.length !== 4) return hhmm;
  return `${Number(hhmm.slice(0, 2))}:${hhmm.slice(2)}`;
}

const styles = StyleSheet.create({
  card: {
    flex: 1,
    backgroundColor: "#F2F2F7",
    borderRadius: 16,
    padding: 16,
  },
  headerRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  label: { fontSize: 15, fontWeight: "600", color: "#8E8E93" },
  time: { fontSize: 13, color: "#8E8E93" },
  umbrella: { fontSize: 28, marginTop: 8 },
  temp: { fontSize: 34, fontWeight: "300", marginTop: 2 },
  meta: { fontSize: 13, color: "#3C3C43", marginTop: 2 },
  hint: { fontSize: 12, color: "#007AFF", marginTop: 10 },
  hourlyBox: {
    marginTop: 12,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderTopColor: "#C6C6C8",
    paddingTop: 8,
  },
  hourlyRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingVertical: 4,
  },
  hourlyTime: { fontSize: 13, color: "#3C3C43", width: 48 },
  hourlyTemp: { fontSize: 13, color: "#000", width: 36, textAlign: "right" },
  hourlyPop: { fontSize: 13, color: "#007AFF", width: 40, textAlign: "right" },
  hourlyPty: { fontSize: 12, color: "#8E8E93", flex: 1, textAlign: "right" },
});
