import { useCallback, useEffect, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ActivityIndicator,
  StyleSheet,
} from "react-native";
import CommuteCard from "../components/CommuteCard";
import HourlySheet from "../components/HourlySheet";
import { loadSettings } from "../storage/settings";
import { getPushToken } from "../lib/push";
import { getForecast, NOT_REGISTERED } from "../lib/api";
import { formatHHmm, dayLabel } from "../lib/format";
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
  // 어느 슬롯의 시간별 시트가 열렸는가. null 이면 닫힘.
  const [sheetSlot, setSheetSlot] = useState<null | "morning" | "evening">(null);

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
          sheetSlot={sheetSlot}
          onOpenSheet={(slot) => setSheetSlot(slot)}
          onCloseSheet={() => setSheetSlot(null)}
        />
      )}
    </View>
  );
}

function ReadyView({
  forecast,
  settings,
  sheetSlot,
  onOpenSheet,
  onCloseSheet,
}: {
  forecast: ForecastResponse;
  settings: Settings;
  sheetSlot: null | "morning" | "evening";
  onOpenSheet: (slot: "morning" | "evening") => void;
  onCloseSheet: () => void;
}) {
  const { morning, evening } = forecast;
  // null 슬롯은 우산 판정에서 제외.
  const needUmbrella =
    (morning?.needUmbrella ?? false) || (evening?.needUmbrella ?? false);

  const { conclusion, subtitle } = mainMessage(settings, morning, evening);

  return (
    <View style={styles.readyContainer}>
      {/* 상단 단일 결론: 화면 위쪽~중앙(황금비 지점)에 배치 */}
      <View style={styles.conclusion}>
        <Text style={styles.conclusionIcon}>{needUmbrella ? "☔️" : "🌤"}</Text>
        <Text style={styles.conclusionText}>{conclusion}</Text>
        <Text style={styles.subtitle}>{subtitle}</Text>
      </View>

      {/* 하단 출근/퇴근 두 카드: 결론 아래 적절한 위치에 */}
      <View style={styles.cards}>
        <CommuteCard
          label="출근"
          day={dayLabel(settings.commuteStart)}
          time={formatHHmm(settings.commuteStart)}
          dong={settings.homeDong}
          data={morning}
          onPress={() => onOpenSheet("morning")}
        />
        <View style={{ width: 12 }} />
        <CommuteCard
          label="퇴근"
          day={dayLabel(settings.commuteEnd)}
          time={formatHHmm(settings.commuteEnd)}
          dong={settings.workDong}
          data={evening}
          onPress={() => onOpenSheet("evening")}
        />
      </View>

      <View style={styles.bottomSpacer} />

      <HourlySheet
        visible={sheetSlot === "morning"}
        title="출근 시간대"
        hourly={morning?.hourly ?? null}
        onClose={onCloseSheet}
      />
      <HourlySheet
        visible={sheetSlot === "evening"}
        title="퇴근 시간대"
        hourly={evening?.hourly ?? null}
        onClose={onCloseSheet}
      />
    </View>
  );
}

// 상단 메시지(결론 + 부연)를 만든다.
//  - 결론: "{오늘/내일}은 우산 {챙기세요/필요 없어요}" (날짜 통합).
//  - 부연: 어디서 비 오는지("출근길에 비가 와요" / "퇴근길에 비가 와요" / "하루 종일 비가 와요" / "우산 없이 가벼워요").
// 우산 판정은 하루 단위(출근 OR 퇴근 중 하나라도 비). null 슬롯은 판정 제외.
//
// 날짜 접두사: 출근 전이면 둘 다 오늘 → "오늘", 퇴근 후면 둘 다 내일 → "내일".
// 출근~퇴근 사이(애매 구간)는 두 카드의 날짜가 갈리는데, 이 경우는 추후 두 줄 처리 예정이라
// 지금은 가까운 쪽(퇴근=오늘 기준)으로 "오늘"을 쓴다.
function mainMessage(
  settings: Settings,
  morning: ForecastResponse["morning"],
  evening: ForecastResponse["evening"]
): { conclusion: string; subtitle: string } {
  const m = morning?.needUmbrella ?? false;
  const e = evening?.needUmbrella ?? false;
  const needUmbrella = m || e;

  // 두 카드의 날짜 라벨. 같으면 그 날, 다르면(애매 구간) "오늘"로.
  const startDay = dayLabel(settings.commuteStart);
  const endDay = dayLabel(settings.commuteEnd);
  const dayWord = startDay === endDay ? startDay : "오늘";

  const conclusion = needUmbrella
    ? `${dayWord}은 우산 챙기세요`
    : `${dayWord}은 우산 필요 없어요`;

  let subtitle: string;
  if (m && e) subtitle = "하루 종일 비가 와요";
  else if (m) subtitle = "출근길에 비가 와요";
  else if (e) subtitle = "퇴근길에 비가 와요";
  else subtitle = "우산 없이 가벼워요";

  return { conclusion, subtitle };
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
  // 결론은 화면 위쪽~중앙(황금비 지점)에 오도록 위 여백을 더 크게.
  conclusion: {
    flex: 0.62,
    alignItems: "center",
    justifyContent: "center",
  },
  conclusionIcon: { fontSize: 72 },
  conclusionText: { fontSize: 34, fontWeight: "700", marginTop: 16, textAlign: "center" },
  subtitle: { fontSize: 17, color: "#8E8E93", marginTop: 8, textAlign: "center" },
  // 카드는 결론 바로 아래에 자리. 두 카드 높이는 CommuteCard 고정 높이로 동일.
  cards: {
    flexDirection: "row",
    alignItems: "stretch",
  },
  // 카드 아래 남는 공간(결론을 위로 끌어올리는 균형추).
  bottomSpacer: { flex: 0.38 },
});
