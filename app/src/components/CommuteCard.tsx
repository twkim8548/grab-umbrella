import { View, Text, Pressable, StyleSheet } from "react-native";
import type { SlotForecast } from "../lib/types";

// 출근/퇴근 카드 (보조 정보). spec §7.1: 우산 여부·기온·강수확률을 압축.
// 메인 상단 결론이 주인공이므로 카드는 절제된 크기로.
// 탭하면 상위(MainScreen)가 시간별 흐름을 하단 시트로 연다. hourly 없으면 탭 비활성.
export default function CommuteCard({
  label,
  time,
  dong,
  data,
  onPress,
}: {
  label: string; // "출근" | "퇴근"
  time: string; // "8:30" 표시용
  dong?: string; // 동네 (예: "역삼동"). 빈 문자열이면 표시 생략.
  data: SlotForecast | null;
  onPress: () => void;
}) {
  const hasHourly = !!data?.hourly && data.hourly.length > 0;

  return (
    <Pressable
      style={styles.card}
      onPress={hasHourly ? onPress : undefined}
      disabled={!hasHourly}
    >
      <View style={styles.headerRow}>
        <Text style={styles.label}>{label}</Text>
        <Text style={styles.time}>{time}</Text>
      </View>
      {dong ? (
        <Text style={styles.dong} numberOfLines={1}>
          {dong}
        </Text>
      ) : null}

      {data ? (
        <>
          <Text style={styles.umbrella}>{data.needUmbrella ? "☔️" : "🌤"}</Text>
          <Text style={styles.temp}>{Math.round(data.tempC)}°</Text>
          <Text style={styles.meta} numberOfLines={1}>
            {data.skyText} · {data.popPct}%
          </Text>

          <Text style={[styles.hint, !hasHourly && styles.hintDisabled]}>
            {hasHourly ? "시간별 보기 ›" : "시간별 정보 없음"}
          </Text>
        </>
      ) : (
        <>
          <Text style={styles.umbrella}>·</Text>
          <Text style={styles.meta}>정보 없음</Text>
          <Text style={[styles.hint, styles.hintDisabled]}>시간별 정보 없음</Text>
        </>
      )}
    </Pressable>
  );
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
  dong: { fontSize: 13, color: "#8E8E93", marginTop: 2 },
  umbrella: { fontSize: 28, marginTop: 10 },
  temp: { fontSize: 34, fontWeight: "300", marginTop: 2 },
  meta: { fontSize: 13, color: "#3C3C43", marginTop: 2 },
  hint: { fontSize: 12, color: "#007AFF", marginTop: 12 },
  hintDisabled: { color: "#C7C7CC" },
});
