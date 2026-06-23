import { View, Text, StyleSheet } from "react-native";
import type { SlotForecast } from "../lib/types";

// 출근/퇴근 카드. spec §7.1: 우산 여부·기온·강수확률·어제 대비 체감을 압축.
// 점진적 공개(탭 시 시간별 흐름)는 추후 추가.
export default function CommuteCard({
  label,
  data,
}: {
  label: string; // "출근" | "퇴근"
  data?: SlotForecast;
}) {
  return (
    <View style={styles.card}>
      <Text style={styles.label}>{label}</Text>
      {data ? (
        <>
          <Text style={styles.umbrella}>
            {data.needUmbrella ? "☔️ 우산 챙기세요" : "🌤 우산 필요 없어요"}
          </Text>
          <Text style={styles.temp}>{data.tempC}°</Text>
          <Text style={styles.meta}>
            {data.skyText} · 강수확률 {data.popPct}%
          </Text>
          {data.feelsVsYesterday ? (
            <Text style={styles.feels}>{data.feelsVsYesterday}</Text>
          ) : null}
        </>
      ) : (
        <Text style={styles.meta}>불러오는 중…</Text>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: "#F2F2F7",
    borderRadius: 16,
    padding: 20,
    marginVertical: 8,
  },
  label: { fontSize: 15, fontWeight: "600", color: "#8E8E93" },
  umbrella: { fontSize: 22, fontWeight: "700", marginTop: 8 },
  temp: { fontSize: 48, fontWeight: "200", marginTop: 4 },
  meta: { fontSize: 15, color: "#3C3C43", marginTop: 4 },
  feels: { fontSize: 15, color: "#007AFF", marginTop: 4 },
});
