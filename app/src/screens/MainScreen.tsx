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
import { formatHHmm } from "../lib/format";
import type { DayForecast, ForecastResponse, Settings, SlotForecast } from "../lib/types";

// 메인 화면: 상단 결론(한 줄/두 줄) + 출근/퇴근 두 카드.
// docs/design-main-screen.md "시간대별 처리". 우산 판정은 하루 단위(그 날 살아있는 슬롯 중 하나라도 비).
// 한 줄(오늘 또는 내일) / 두 줄(출퇴근 사이: 오늘+내일)은 살아있는 슬롯 데이터로 결정.
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

// 출근/퇴근 카드용으로 "지금 시점 기준 다음 시점"을 고른다.
// today 슬롯이 살아있으면(=null 아님) 그것(오늘), 아니면 tomorrow 슬롯(내일).
// day 라벨은 시각 계산이 아니라 어느 날 슬롯을 골랐는지로 정한다.
function pickNextSlot(
  forecast: ForecastResponse,
  key: "morning" | "evening"
): { data: SlotForecast | null; day: "오늘" | "내일" } {
  const todaySlot = forecast.today[key];
  if (todaySlot) return { data: todaySlot, day: "오늘" };
  return { data: forecast.tomorrow[key], day: "내일" };
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
  // 카드: 지금 기준 "다음 출근/다음 퇴근" (today 우선, 없으면 tomorrow).
  const next = {
    morning: pickNextSlot(forecast, "morning"),
    evening: pickNextSlot(forecast, "evening"),
  };

  const message = mainMessage(forecast);

  return (
    <View style={styles.readyContainer}>
      {/* 상단 결론: 한 줄(큰 결론) 또는 두 줄(오늘+내일 각각). */}
      <View style={styles.conclusion}>
        <Text style={styles.conclusionIcon}>{message.needUmbrella ? "☔️" : "🌤"}</Text>
        {message.lines.length === 1 ? (
          <>
            <Text style={styles.conclusionText}>{message.lines[0].conclusion}</Text>
            <Text style={styles.subtitle}>{message.lines[0].subtitle}</Text>
          </>
        ) : (
          <View style={styles.twoLines}>
            {message.lines.map((line) => (
              <View key={line.dayWord} style={styles.lineBlock}>
                <Text style={styles.lineConclusion}>{line.conclusion}</Text>
                <Text style={styles.lineSubtitle}>{line.subtitle}</Text>
              </View>
            ))}
          </View>
        )}
      </View>

      {/* 하단 출근/퇴근 두 카드: 결론 아래 적절한 위치에 */}
      <View style={styles.cards}>
        <CommuteCard
          label="출근"
          day={next.morning.day}
          time={formatHHmm(settings.commuteStart)}
          dong={settings.homeDong}
          data={next.morning.data}
          onPress={() => onOpenSheet("morning")}
        />
        <View style={{ width: 12 }} />
        <CommuteCard
          label="퇴근"
          day={next.evening.day}
          time={formatHHmm(settings.commuteEnd)}
          dong={settings.workDong}
          data={next.evening.data}
          onPress={() => onOpenSheet("evening")}
        />
      </View>

      <View style={styles.bottomSpacer} />

      <HourlySheet
        visible={sheetSlot === "morning"}
        title="출근 시간대"
        hourly={next.morning.data?.hourly ?? null}
        onClose={onCloseSheet}
      />
      <HourlySheet
        visible={sheetSlot === "evening"}
        title="퇴근 시간대"
        hourly={next.evening.data?.hourly ?? null}
        onClose={onCloseSheet}
      />
    </View>
  );
}

interface MessageLine {
  dayWord: "오늘" | "내일";
  conclusion: string; // "오늘은 우산 챙기세요"
  subtitle: string; // 부연 ("출근길에 비가 와요" 등)
}

// 한 날(today/tomorrow)의 결론 한 줄을 만든다.
// 우산 판정 = 그 날 살아있는(null 아닌) 슬롯 중 하나라도 needUmbrella.
// 부연: 어느 시점이 비인지(출근길/퇴근길/하루 종일/안 비).
function dayConclusion(dayWord: "오늘" | "내일", day: DayForecast): MessageLine {
  const m = day.morning?.needUmbrella ?? false;
  const e = day.evening?.needUmbrella ?? false;
  const needUmbrella = m || e;

  const conclusion = needUmbrella
    ? `${dayWord}은 우산 챙기세요`
    : `${dayWord}은 우산 필요 없어요`;

  let subtitle: string;
  if (m && e) subtitle = `${dayWord} 하루 종일 비가 와요`;
  else if (m) subtitle = `${dayWord} 출근길엔 비가 와요`;
  else if (e) subtitle = `${dayWord} 퇴근길엔 비가 와요`;
  else subtitle = "우산 없이 가벼워요";

  return { dayWord, conclusion, subtitle };
}

// 상단 메시지를 만든다. 데이터(살아있는 슬롯)로 한 줄/두 줄을 결정한다.
//  - 출근 전: today.morning + today.evening 살아있음 → 오늘 한 줄.
//  - 출퇴근 사이: today.morning=null(출근 지남) + today.evening 살아있음 + tomorrow 살아있음 → 두 줄(오늘+내일).
//  - 퇴근 후: today 전부 null, tomorrow 만 살아있음 → 내일 한 줄.
// needUmbrella: 보여주는 줄들 중 하나라도 비면 큰 아이콘 ☔️.
function mainMessage(forecast: ForecastResponse): {
  lines: MessageLine[];
  needUmbrella: boolean;
} {
  const { today, tomorrow } = forecast;
  const todayAlive = !!(today.morning || today.evening);
  const tomorrowAlive = !!(tomorrow.morning || tomorrow.evening);

  // 출퇴근 사이(애매 구간): 출근 지나(today.morning=null) 오늘 일부만 남고 내일도 관심사 → 두 줄.
  const twoLines = todayAlive && tomorrowAlive && !today.morning;

  let lines: MessageLine[];
  if (twoLines) {
    lines = [dayConclusion("오늘", today), dayConclusion("내일", tomorrow)];
  } else if (todayAlive) {
    lines = [dayConclusion("오늘", today)];
  } else {
    lines = [dayConclusion("내일", tomorrow)];
  }

  const dayNeedsUmbrella = (d: DayForecast) =>
    (d.morning?.needUmbrella ?? false) || (d.evening?.needUmbrella ?? false);
  const needUmbrella = twoLines
    ? dayNeedsUmbrella(today) || dayNeedsUmbrella(tomorrow)
    : dayNeedsUmbrella(todayAlive ? today : tomorrow);

  return { lines, needUmbrella };
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
  // 두 줄(오늘+내일): 한 줄 큰 결론보다 절제된 중간 크기로 균형. HIG 톤.
  twoLines: { marginTop: 16, alignItems: "center", gap: 16 },
  lineBlock: { alignItems: "center" },
  lineConclusion: { fontSize: 24, fontWeight: "700", textAlign: "center" },
  lineSubtitle: { fontSize: 15, color: "#8E8E93", marginTop: 4, textAlign: "center" },
  // 카드는 결론 바로 아래에 자리. 두 카드 높이는 CommuteCard 고정 높이로 동일.
  cards: {
    flexDirection: "row",
    alignItems: "stretch",
  },
  // 카드 아래 남는 공간(결론을 위로 끌어올리는 균형추).
  bottomSpacer: { flex: 0.38 },
});
