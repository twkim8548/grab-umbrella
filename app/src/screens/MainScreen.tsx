import { useCallback, useEffect, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ActivityIndicator,
  ScrollView,
  StyleSheet,
} from "react-native";
import CommuteCard from "../components/CommuteCard";
import HourlySheet from "../components/HourlySheet";
import { loadSettings } from "../storage/settings";
import { getPushToken } from "../lib/push";
import { getForecast, NOT_REGISTERED } from "../lib/api";
import { formatHHmm } from "../lib/format";
import type { DayForecast, ForecastResponse, Settings, SlotForecast } from "../lib/types";

// 메인 화면: "날짜 섹션" 패러다임. 각 섹션 = [날짜 헤더 + 그 날 우산 결론] + [그 날 살아있는 카드].
// docs/design-main-screen.md "시간대별 처리". 우산 판정은 하루 단위(그 날 살아있는 슬롯 중 하나라도 비).
// 섹션 개수만 구간에 따라 1~2개로 달라진다(출근 전=오늘1, 출퇴근 사이=오늘+내일, 퇴근 후=내일1).
type LoadState =
  | { kind: "loading" }
  | { kind: "no-settings" }
  | { kind: "sync-needed" }
  | { kind: "error"; message: string }
  | { kind: "ready"; settings: Settings; forecast: ForecastResponse };

type DayWord = "오늘" | "내일";
type SlotKey = "morning" | "evening";
// 시트 식별: 어느 섹션(날짜)의 어느 슬롯이 열렸는가. null 이면 닫힘.
type SheetTarget = { dayWord: DayWord; slotKey: SlotKey } | null;

export default function MainScreen({ onOpenSettings }: { onOpenSettings: () => void }) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [sheet, setSheet] = useState<SheetTarget>(null);

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
          sheet={sheet}
          onOpenSheet={(dayWord, slotKey) => setSheet({ dayWord, slotKey })}
          onCloseSheet={() => setSheet(null)}
        />
      )}
    </View>
  );
}

// 표시할 날짜 섹션 목록을 데이터로 결정한다.
//  - 출근 전: today.morning 살아있음 → "오늘"만(내일 안 보임).
//  - 출퇴근 사이: today.morning=null(출근 지남) + today.evening 살아있음 + tomorrow 살아있음 → "오늘"+"내일".
//  - 퇴근 후: today 전부 null → "내일"만.
function pickSections(forecast: ForecastResponse): DayWord[] {
  const { today, tomorrow } = forecast;
  const todayAlive = !!(today.morning || today.evening);
  const tomorrowAlive = !!(tomorrow.morning || tomorrow.evening);

  const sections: DayWord[] = [];
  if (todayAlive) {
    sections.push("오늘");
    // 오늘이 "완전한 하루"가 아니면(출근 지남) 다가오는 내일도 관심사.
    if (!today.morning && tomorrowAlive) sections.push("내일");
  } else {
    sections.push("내일");
  }
  return sections;
}

function ReadyView({
  forecast,
  settings,
  sheet,
  onOpenSheet,
  onCloseSheet,
}: {
  forecast: ForecastResponse;
  settings: Settings;
  sheet: SheetTarget;
  onOpenSheet: (dayWord: DayWord, slotKey: SlotKey) => void;
  onCloseSheet: () => void;
}) {
  const sections = pickSections(forecast);
  const dayOf = (w: DayWord): DayForecast => (w === "오늘" ? forecast.today : forecast.tomorrow);

  // 열린 시트의 hourly/title 은 섹션×슬롯 식별자로 역참조.
  const sheetData: SlotForecast | null = sheet ? dayOf(sheet.dayWord)[sheet.slotKey] : null;
  const sheetTitle = sheet
    ? `${sheet.dayWord} ${sheet.slotKey === "morning" ? "출근" : "퇴근"}`
    : "";

  return (
    <View style={styles.readyContainer}>
      <ScrollView
        contentContainerStyle={styles.scrollContent}
        showsVerticalScrollIndicator={false}
      >
        {sections.map((dayWord) => (
          <DaySection
            key={dayWord}
            dayWord={dayWord}
            dayForecast={dayOf(dayWord)}
            settings={settings}
            onOpenSheet={onOpenSheet}
          />
        ))}

        {/* 데이터 출처 표기. 공공데이터(기상청 단기예보) 이용 명시. */}
        <Text style={styles.source}>기상청 제공</Text>
      </ScrollView>

      <HourlySheet
        visible={!!sheet}
        title={sheetTitle}
        hourly={sheetData?.hourly ?? null}
        onClose={onCloseSheet}
      />
    </View>
  );
}

// 한 날짜 섹션: 날짜 헤더 + 결론 한 줄 + 그 날 살아있는 카드들(가로 배치).
// 우산 판정 = 그 날 살아있는 슬롯 중 하나라도 needUmbrella.
function DaySection({
  dayWord,
  dayForecast,
  settings,
  onOpenSheet,
}: {
  dayWord: DayWord;
  dayForecast: DayForecast;
  settings: Settings;
  onOpenSheet: (dayWord: DayWord, slotKey: SlotKey) => void;
}) {
  const needUmbrella =
    (dayForecast.morning?.needUmbrella ?? false) ||
    (dayForecast.evening?.needUmbrella ?? false);

  // 카드 자리: 출근/퇴근 두 칸을 항상 둔다.
  //  - 오늘 섹션: 지난 슬롯(null)은 흐린 "지났어요" 카드로 자리를 채워 두 칸 균형 유지.
  //  - 내일 섹션: 지난 게 없으므로 살아있는 슬롯만(보통 둘 다).
  const slots: { key: SlotKey; label: string; time: string; dong: string }[] = [
    { key: "morning", label: "출근", time: formatHHmm(settings.commuteStart), dong: settings.homeDong },
    { key: "evening", label: "퇴근", time: formatHHmm(settings.commuteEnd), dong: settings.workDong },
  ];
  const isToday = dayWord === "오늘";
  // 그릴 슬롯: 데이터 있으면 정상, 없으면 오늘 섹션에서만 past 카드로.
  const cards = slots
    .map((s) => {
      const data = dayForecast[s.key];
      if (data) return { ...s, data, past: false };
      if (isToday) return { ...s, data: null as SlotForecast | null, past: true };
      return null;
    })
    .filter((c): c is NonNullable<typeof c> => c !== null);
  const single = cards.length === 1;

  return (
    <View style={styles.section}>
      <Text style={styles.dayWord}>{dayWord}</Text>
      <View style={styles.conclusionRow}>
        <Text style={styles.conclusionIcon}>{needUmbrella ? "☔️" : "🌤"}</Text>
        <Text style={styles.conclusionText}>
          {needUmbrella ? "우산 챙기세요" : "우산 필요 없어요"}
        </Text>
      </View>

      {/* 카드 1개면 가로 절반만 차지(혼자 꽉 차지 않게), 2개면 동등 분할. */}
      <View style={styles.cards}>
        {cards.map((c, i) => (
          <View
            key={c.key}
            style={[styles.cardSlot, single && styles.cardSlotSingle, i > 0 && styles.cardGap]}
          >
            <CommuteCard
              label={c.label}
              time={c.time}
              dong={c.dong}
              data={c.data}
              past={c.past}
              onPress={() => onOpenSheet(dayWord, c.key)}
            />
          </View>
        ))}
      </View>
    </View>
  );
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
  scrollContent: { paddingTop: 8, paddingBottom: 24 },
  source: { fontSize: 12, color: "#C7C7CC", textAlign: "center", marginTop: 24 },
  // 섹션: 날짜 헤더 + 결론 + 카드. 섹션 간 넉넉한 간격.
  section: { marginBottom: 28 },
  dayWord: { fontSize: 24, fontWeight: "700" },
  conclusionRow: {
    flexDirection: "row",
    alignItems: "center",
    marginTop: 6,
    marginBottom: 14,
  },
  conclusionIcon: { fontSize: 28, marginRight: 8 },
  conclusionText: { fontSize: 20, fontWeight: "600", color: "#3C3C43" },
  // 카드 행: 살아있는 카드들을 가로로.
  cards: { flexDirection: "row", alignItems: "stretch" },
  cardSlot: { flex: 1 },
  // 카드 1개일 때는 가로 절반만 차지(혼자 화면을 꽉 채우지 않게).
  cardSlotSingle: { flex: 0, width: "50%" },
  cardGap: { marginLeft: 12 },
});
