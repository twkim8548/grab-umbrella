import { View, Text, Pressable, StyleSheet } from "react-native";
import type { SlotForecast } from "../lib/types";

// 그 시각(anchor)의 실제 날씨 이모지. 강수형태(PTY)가 있으면 우선, 없으면 하늘상태(SKY)로.
// 우산 필요 여부와는 무관하게 "지금 이 시각 하늘"만 그린다(아이콘=날씨, 우산=부제목).
function weatherEmoji(ptyText: string, skyText: string): string {
  switch (ptyText) {
    case "비":
      return "🌧";
    case "비/눈":
      return "🌨";
    case "눈":
      return "❄️";
    case "소나기":
      return "🌦";
  }
  // 강수 없음 → 하늘상태로.
  switch (skyText) {
    case "맑음":
      return "☀️";
    case "구름많음":
      return "⛅️";
    case "흐림":
      return "☁️";
    default:
      return "🌤";
  }
}

// 출근/퇴근 카드 (보조 정보). spec §7.1: 우산 여부·기온·강수확률을 압축.
// 메인 상단 결론이 주인공이므로 카드는 절제된 크기로.
// 탭하면 상위(MainScreen)가 시간별 흐름을 하단 시트로 연다. hourly 없으면 탭 비활성.
export default function CommuteCard({
  label,
  day,
  time,
  dong,
  data,
  past,
  onPress,
}: {
  label: string; // "출근" | "퇴근"
  day?: string; // "오늘" | "내일". 시각 옆에 작게.
  time: string; // "8:30" 표시용
  dong?: string; // 동네 (예: "역삼동"). 빈 문자열이면 표시 생략.
  data: SlotForecast | null;
  past?: boolean; // 이미 지난 시점(예보 없음). 흐리게 "지남" 표시.
  onPress: () => void;
}) {
  const hasHourly = !!data?.hourly && data.hourly.length > 0;

  // 이미 지난 시점: 예보가 없으므로 흐린 비활성 카드로 자리만 채운다.
  if (past) {
    return (
      <View style={[styles.card, styles.cardPast]}>
        <View style={styles.headerRow}>
          <Text style={styles.label}>{label}</Text>
          <Text style={styles.time}>{time}</Text>
        </View>
        {dong ? (
          <Text style={styles.dong} numberOfLines={1}>
            {dong}
          </Text>
        ) : null}
        <View style={styles.pastFill}>
          <Text style={styles.pastText}>지났어요</Text>
        </View>
      </View>
    );
  }

  return (
    <Pressable
      style={styles.card}
      onPress={hasHourly ? onPress : undefined}
      disabled={!hasHourly}
    >
      <View style={styles.headerRow}>
        <Text style={styles.label}>{label}</Text>
        <Text style={styles.time}>
          {day ? `${day} ` : ""}
          {time}
        </Text>
      </View>
      {dong ? (
        <Text style={styles.dong} numberOfLines={1}>
          {dong}
        </Text>
      ) : null}

      {data ? (
        <>
          {/* 메인 아이콘은 우산 여부가 아니라 그 시각(anchor)의 실제 날씨를 그린다.
              우산 필요는 아래 부제목으로 전달 → "18시 맑은데 ☔️" 모순 제거. */}
          <Text style={styles.umbrella}>{weatherEmoji(data.ptyText, data.skyText)}</Text>
          <Text style={styles.temp}>{Math.round(data.tempC)}°</Text>
          <Text style={styles.meta} numberOfLines={1}>
            {data.skyText} · {data.popPct}%
          </Text>

          {/* 대표값(맑음)과 우산 결론이 어긋날 때의 근거. 예: "19시부터 소나기".
              혼란("맑은데 왜 우산?")을 없애기 위해 카드에서 직접 이유를 보여준다. */}
          {data.needUmbrella && data.umbrellaReason ? (
            <Text style={styles.reason} numberOfLines={1}>
              ☔️ {data.umbrellaReason}
            </Text>
          ) : null}

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
  // 우산 근거 부제목: 대표값과 어긋날 때만 표시. 비를 알리는 강조 톤.
  reason: { fontSize: 12, fontWeight: "600", color: "#0A84FF", marginTop: 6 },
  hint: { fontSize: 12, color: "#007AFF", marginTop: 12 },
  hintDisabled: { color: "#C7C7CC" },
  // 지난 시점 카드: 흐리게, 자리만 채움.
  cardPast: { opacity: 0.5 },
  pastFill: { flex: 1, justifyContent: "center", paddingVertical: 24 },
  pastText: { fontSize: 15, color: "#8E8E93" },
});
