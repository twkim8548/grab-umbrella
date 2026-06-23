import { useCallback, useEffect, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ActivityIndicator,
  StyleSheet,
} from "react-native";
import CommuteCard from "../components/CommuteCard";
import { loadSettings } from "../storage/settings";
import { getPushToken } from "../lib/push";
import { getForecast, NOT_REGISTERED } from "../lib/api";
import type { ForecastResponse, Settings } from "../lib/types";

// 메인 화면: 상단 단일 결론 + 부연 한 줄 + 출근/퇴근 두 카드.
// docs/design-main-screen.md 확정안. 우산 판정 = morning.needUmbrella || evening.needUmbrella.
type LoadState =
  | { kind: "loading" }
  | { kind: "no-settings" }
  | { kind: "sync-needed" }
  | { kind: "error"; message: string }
  | { kind: "ready"; settings: Settings; forecast: ForecastResponse };

export default function MainScreen({ onOpenSettings }: { onOpenSettings: () => void }) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [expanded, setExpanded] = useState<null | "morning" | "evening">(null);

  const load = useCallback(async () => {
    setState({ kind: "loading" });
    try {
      const settings = await loadSettings();
      if (!settings) {
        setState({ kind: "no-settings" });
        return;
      }
      // 토큰은 권한과 무관하게 항상 발급된다(dev 폴백 보장). 메인은 forecast 호출용으로만 사용.
      const token = await getPushToken();
      const forecast = await getForecast(token);
      setState({ kind: "ready", settings, forecast });
    } catch (e) {
      // 404(NOT_REGISTERED): 서버에 미등록. 로컬 설정은 있으므로 "동기화 필요" 안내.
      // (네트워크/5xx 등 실제 오류만 재시도 가능한 error 로.)
      if (e instanceof Error && e.message === NOT_REGISTERED) {
        setState({ kind: "sync-needed" });
        return;
      }
      const message = e instanceof Error ? e.message : "예보를 불러오지 못했습니다.";
      setState({ kind: "error", message });
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>우산챙겨?</Text>
        <Pressable onPress={onOpenSettings} hitSlop={12}>
          <Text style={styles.gear}>⚙︎</Text>
        </Pressable>
      </View>

      {state.kind === "loading" ? (
        <View style={styles.center}>
          <ActivityIndicator />
        </View>
      ) : state.kind === "no-settings" ? (
        <View style={styles.center}>
          <Text style={styles.emptyText}>설정에서 위치를 등록하세요.</Text>
          <Pressable style={styles.primaryButton} onPress={onOpenSettings}>
            <Text style={styles.primaryButtonText}>설정으로 이동</Text>
          </Pressable>
        </View>
      ) : state.kind === "sync-needed" ? (
        <View style={styles.center}>
          <Text style={styles.emptyText}>
            서버에 동기화가 안 됐어요.{"\n"}설정에서 다시 저장해주세요.
          </Text>
          <Pressable style={styles.primaryButton} onPress={onOpenSettings}>
            <Text style={styles.primaryButtonText}>설정으로 이동</Text>
          </Pressable>
        </View>
      ) : state.kind === "error" ? (
        <View style={styles.center}>
          <Text style={styles.emptyText}>{state.message}</Text>
          <Pressable style={styles.primaryButton} onPress={load}>
            <Text style={styles.primaryButtonText}>다시 시도</Text>
          </Pressable>
        </View>
      ) : (
        <ReadyView
          forecast={state.forecast}
          settings={state.settings}
          expanded={expanded}
          onToggle={(slot) => setExpanded((cur) => (cur === slot ? null : slot))}
        />
      )}
    </View>
  );
}

function ReadyView({
  forecast,
  settings,
  expanded,
  onToggle,
}: {
  forecast: ForecastResponse;
  settings: Settings;
  expanded: null | "morning" | "evening";
  onToggle: (slot: "morning" | "evening") => void;
}) {
  const { morning, evening } = forecast;
  // null 슬롯은 우산 판정에서 제외.
  const needUmbrella =
    (morning?.needUmbrella ?? false) || (evening?.needUmbrella ?? false);

  return (
    <View style={styles.readyContainer}>
      {/* 상단 단일 결론 */}
      <View style={styles.conclusion}>
        <Text style={styles.conclusionIcon}>{needUmbrella ? "☔️" : "🌤"}</Text>
        <Text style={styles.conclusionText}>
          {needUmbrella ? "우산 챙기세요" : "우산 필요 없어요"}
        </Text>
        <Text style={styles.subtitle}>{subtitleFor(morning, evening)}</Text>
      </View>

      {/* 하단 출근/퇴근 두 카드 */}
      <View style={styles.cards}>
        <CommuteCard
          label="출근"
          time={formatTime(settings.commuteStart)}
          data={morning}
          expanded={expanded === "morning"}
          onToggle={() => onToggle("morning")}
        />
        <View style={{ width: 12 }} />
        <CommuteCard
          label="퇴근"
          time={formatTime(settings.commuteEnd)}
          data={evening}
          expanded={expanded === "evening"}
          onToggle={() => onToggle("evening")}
        />
      </View>
    </View>
  );
}

// 부연 한 줄: 어느 시점이 비인지 정직하게. null 슬롯은 판정 제외.
function subtitleFor(
  morning: ForecastResponse["morning"],
  evening: ForecastResponse["evening"]
): string {
  const m = morning?.needUmbrella ?? false;
  const e = evening?.needUmbrella ?? false;
  if (m && e) return "오늘 종일 비 소식";
  if (m) return "출근길에 비가 와요";
  if (e) return "퇴근길에 비가 와요";
  return "오늘은 우산 없이 가벼워요";
}

// "0830" → "8:30"
function formatTime(hhmm: string): string {
  if (hhmm.length !== 4) return hhmm;
  return `${Number(hhmm.slice(0, 2))}:${hhmm.slice(2)}`;
}

const styles = StyleSheet.create({
  container: { flex: 1, paddingHorizontal: 20, paddingTop: 8 },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: 12,
  },
  title: { fontSize: 28, fontWeight: "700" },
  gear: { fontSize: 24 },
  center: { flex: 1, alignItems: "center", justifyContent: "center", gap: 16 },
  emptyText: { fontSize: 17, color: "#3C3C43", textAlign: "center", paddingHorizontal: 24 },
  primaryButton: {
    backgroundColor: "#007AFF",
    borderRadius: 12,
    paddingVertical: 14,
    paddingHorizontal: 28,
  },
  primaryButtonText: { color: "#fff", fontSize: 17, fontWeight: "600" },
  readyContainer: { flex: 1 },
  conclusion: { flex: 1, alignItems: "center", justifyContent: "center" },
  conclusionIcon: { fontSize: 72 },
  conclusionText: { fontSize: 34, fontWeight: "700", marginTop: 16, textAlign: "center" },
  subtitle: { fontSize: 17, color: "#8E8E93", marginTop: 8, textAlign: "center" },
  cards: {
    flexDirection: "row",
    alignItems: "flex-start",
    marginBottom: 12,
  },
});
