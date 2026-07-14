import { useCallback, useRef, useState } from "react";
import {
  View,
  Text,
  Pressable,
  ActivityIndicator,
  ScrollView,
  RefreshControl,
  StyleSheet,
} from "react-native";
import { useFocusEffect } from "@react-navigation/native";
import CommuteCard from "../components/CommuteCard";
import HourlySheet from "../components/HourlySheet";
import { loadSettings } from "../storage/settings";
import { loadForecastCache, saveForecastCache } from "../storage/forecast";
import { getPushToken } from "../lib/push";
import { getForecast, NOT_REGISTERED, sync } from "../lib/api";
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
type UpdateStatus = "idle" | "updating" | "error";

export default function MainScreen({ onOpenSettings }: { onOpenSettings: () => void }) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const [sheet, setSheet] = useState<SheetTarget>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [updateStatus, setUpdateStatus] = useState<UpdateStatus>("idle");
  const loadRequestRef = useRef(0);
  const loadAbortRef = useRef<AbortController | null>(null);
  const hasReadyDataRef = useRef(false);

  // silent=true 면 전체 화면을 loading 스피너로 덮지 않는다(pull-to-refresh 용).
  // 결과는 동일하게 ready/sync-needed/error 로 귀결된다.
  const load = useCallback(async (silent = false) => {
    loadAbortRef.current?.abort();
    const controller = new AbortController();
    loadAbortRef.current = controller;
    const requestId = ++loadRequestRef.current;
    const isLatest = () =>
      requestId === loadRequestRef.current && !controller.signal.aborted;
    let displayedCache = false;
    if (!silent) {
      setUpdateStatus("idle");
      setState({ kind: "loading" });
    } else if (hasReadyDataRef.current) {
      setUpdateStatus("updating");
    }
    try {
      const settings = await loadSettings();
      if (!isLatest()) return;
      if (!settings) {
        if (isLatest()) setState({ kind: "no-settings" });
        return;
      }

      // 앱 최초 진입에는 같은 설정의 신선한 예보를 즉시 보여주고, 아래 네트워크
      // 조회는 그대로 계속해 최신 데이터로 교체한다.
      if (!silent) {
        const cachedForecast = await loadForecastCache(settings);
        if (!isLatest()) return;
        if (cachedForecast && isLatest()) {
          displayedCache = true;
          hasReadyDataRef.current = true;
          setUpdateStatus("updating");
          setState({ kind: "ready", settings, forecast: cachedForecast });
        }
      }
      // 토큰은 권한과 무관하게 항상 발급된다(dev 폴백 보장). 메인은 forecast 호출용으로만 사용.
      const token = await getPushToken();
      if (!isLatest()) return;
      let forecast: ForecastResponse;
      try {
        forecast = await getForecast(token, controller.signal);
      } catch (e) {
        if (!(e instanceof Error) || e.message !== NOT_REGISTERED) throw e;

        // 최초 sync 실패, 앱 재설치, dev 토큰→정식 Expo 토큰 전환은 모두 서버에서
        // "미등록 토큰"으로 보인다. 로컬 설정이 주인이므로 한 번 자동 복구한 뒤 재조회한다.
        if (!isLatest()) return;
        await sync(token, settings, controller.signal);
        if (!isLatest()) return;
        forecast = await getForecast(token, controller.signal);
      }
      if (isLatest()) {
        await saveForecastCache(settings, forecast);
      }
      if (isLatest()) {
        hasReadyDataRef.current = true;
        setUpdateStatus("idle");
        setState({ kind: "ready", settings, forecast });
      }
    } catch (e) {
      if (e instanceof Error && e.name === "AbortError") return;
      // Settings 복귀/새로고침으로 더 최신 요청이 시작됐다면 오래된 응답은 버린다.
      if (!isLatest()) return;
      // 이미 표시 중인 예보를 갱신하다 실패했다면 기존 화면을 유지한다. 일시적인
      // Lambda/KMA 지연 때문에 홈 전체가 에러 화면으로 바뀌지 않게 한다.
      if ((silent && hasReadyDataRef.current) || displayedCache) {
        setUpdateStatus("error");
        return;
      }
      // 404(NOT_REGISTERED): 서버에 미등록. 로컬 설정은 있으므로 "동기화 필요" 안내.
      // (네트워크/5xx 등 실제 오류만 재시도 가능한 error 로.)
      if (e instanceof Error && e.message === NOT_REGISTERED) {
        setState({ kind: "sync-needed" });
        return;
      }
      const message = e instanceof Error ? e.message : "예보를 불러오지 못했습니다.";
      setState({ kind: "error", message });
    } finally {
      if (loadAbortRef.current === controller) loadAbortRef.current = null;
    }
  }, []);

  // 아래로 당겨서 새로고침: 화면을 깜빡이지 않고 데이터만 다시 가져온다.
  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await load(true);
    } finally {
      setRefreshing(false);
    }
  }, [load]);

  // MainScreen 은 Settings 가 push 되어도 마운트된 채 남는다. 따라서 최초 mount 에서만
  // 읽으면 설정 저장 후 돌아왔을 때도 이전 no-settings/sync-needed 상태가 남는다.
  // 최초 진입과 Settings 에서 돌아오는 매 focus 마다 로컬 설정과 예보를 다시 읽는다.
  useFocusEffect(
    useCallback(() => {
      // 최초 진입은 전체 로딩을 보여주되, Settings 에서 돌아올 때는 기존 예보를
      // 그대로 둔 채 백그라운드에서 새 설정·예보로 교체한다.
      void load(hasReadyDataRef.current);
      return () => {
        loadAbortRef.current?.abort();
        loadAbortRef.current = null;
        ++loadRequestRef.current;
      };
    }, [load])
  );

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
          <Pressable style={styles.primaryButton} onPress={() => load()}>
            <Text style={styles.primaryButtonText}>다시 시도</Text>
          </Pressable>
        </View>
      ) : (
        <ReadyView
          forecast={state.forecast}
          settings={state.settings}
          sheet={sheet}
          refreshing={refreshing}
          updateStatus={updateStatus}
          onRefresh={onRefresh}
          onRetry={() => void load(true)}
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

// 한 슬롯의 우산 이유 문구를 만든다. 서버 reason(예 "19시부터 소나기")이 있으면 우선,
// 없으면(anchor 자체가 비라 reason 비어 있음) 강수형태(ptyText)로 폴백. 둘 다 없으면 "비".
function slotReason(slot: SlotForecast | null): string {
  if (!slot?.needUmbrella) return "";
  if (slot.umbrellaReason) return slot.umbrellaReason;
  return slot.ptyText && slot.ptyText !== "없음" ? slot.ptyText : "비";
}

// 섹션(하루) 결론 아래에 붙일 이유. 우산이 필요한 슬롯별로 "출근길 …" / "퇴근길 …" 을
// 모아 콤마로 잇는다. 예: "퇴근길 19시부터 소나기" 또는 "출근길 비, 퇴근길 소나기".
function buildSectionReason(day: DayForecast): string {
  const parts: string[] = [];
  const m = slotReason(day.morning);
  if (m) parts.push(`출근길 ${m}`);
  const e = slotReason(day.evening);
  if (e) parts.push(`퇴근길 ${e}`);
  return parts.join(", ");
}

function ReadyView({
  forecast,
  settings,
  sheet,
  refreshing,
  updateStatus,
  onRefresh,
  onRetry,
  onOpenSheet,
  onCloseSheet,
}: {
  forecast: ForecastResponse;
  settings: Settings;
  sheet: SheetTarget;
  refreshing: boolean;
  updateStatus: UpdateStatus;
  onRefresh: () => void;
  onRetry: () => void;
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
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor="#8E8E93" />
        }
      >
        {updateStatus === "updating" ? (
          <View style={styles.updateNotice}>
            <ActivityIndicator size="small" />
            <Text style={styles.updateNoticeText}>날씨 업데이트 중…</Text>
          </View>
        ) : updateStatus === "error" ? (
          <View style={styles.updateNotice}>
            <Text style={styles.updateErrorText}>최신 날씨를 불러오지 못했어요</Text>
            <Pressable onPress={onRetry} hitSlop={8}>
              <Text style={styles.retryText}>다시 시도</Text>
            </Pressable>
          </View>
        ) : null}

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

  // 결론 아래 작은 이유: 우산이 필요한 슬롯별로 "출근길 …" / "퇴근길 …" 을 모은다.
  // 슬롯 reason(예 "19시부터 소나기")이 있으면 그걸, anchor 자체가 비라 비어 있으면
  // 강수형태(ptyText)로 폴백("비"/"소나기"). 우산 불필요면 빈 문자열.
  const reasonText = needUmbrella ? buildSectionReason(dayForecast) : "";

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
      {/* 아이콘 + (결론 텍스트 / 이유) 세로 묶음. 이유를 텍스트와 같은 컨테이너에 넣어
          결론 텍스트와 정확히 같은 x 에서 시작하게 한다(이모지 폭에 의존하지 않음).
          이유가 있으면(2줄) 상단 정렬, 없으면(1줄) 아이콘과 중앙 정렬. */}
      <View style={[styles.conclusionRow, reasonText ? styles.conclusionRowTop : styles.conclusionRowCenter]}>
        <Text style={styles.conclusionIcon}>{needUmbrella ? "☔️" : "🌤"}</Text>
        <View style={styles.conclusionTextCol}>
          <Text style={styles.conclusionText}>
            {needUmbrella ? "우산 챙기세요" : "우산 필요 없어요"}
          </Text>
          {/* 우산이 필요할 때만, 결론 바로 아래에 작게 이유를 보여준다. */}
          {reasonText ? <Text style={styles.conclusionReason}>{reasonText}</Text> : null}
        </View>
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
    marginBottom: 24,
  },
  title: { fontSize: 28, fontWeight: "700" },
  gear: { fontSize: 30 },
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
  updateNotice: {
    minHeight: 28,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 7,
    marginBottom: 12,
  },
  updateNoticeText: { fontSize: 13, color: "#8E8E93" },
  updateErrorText: { fontSize: 13, color: "#8E8E93" },
  retryText: { fontSize: 13, color: "#007AFF", fontWeight: "600" },
  source: { fontSize: 12, color: "#C7C7CC", textAlign: "center", marginTop: 24 },
  // 섹션: 날짜 헤더 + 결론 + 카드. 섹션 간 넉넉한 간격.
  section: { marginBottom: 28 },
  dayWord: { fontSize: 24, fontWeight: "700" },
  conclusionRow: {
    flexDirection: "row",
    marginTop: 6,
    marginBottom: 14,
  },
  // 이유가 있을 때(2줄): 아이콘을 결론 텍스트 첫 줄에 맞춰 상단 정렬.
  conclusionRowTop: { alignItems: "flex-start" },
  // 이유가 없을 때(1줄): 아이콘과 결론 텍스트를 중앙 정렬.
  conclusionRowCenter: { alignItems: "center" },
  conclusionIcon: { fontSize: 28, marginRight: 8 },
  // 결론 텍스트 + 이유를 담는 세로 컬럼. 이유가 텍스트와 같은 x 에서 시작하도록.
  conclusionTextCol: { flex: 1 },
  conclusionText: { fontSize: 20, fontWeight: "600", color: "#3C3C43" },
  // 결론 바로 아래 작은 이유. 같은 컬럼에 있어 들여쓰기가 자동으로 맞는다.
  conclusionReason: { fontSize: 13, color: "#8E8E93", marginTop: 2 },
  // 카드 행: 살아있는 카드들을 가로로.
  cards: { flexDirection: "row", alignItems: "stretch" },
  cardSlot: { flex: 1 },
  // 카드 1개일 때는 가로 절반만 차지(혼자 화면을 꽉 채우지 않게).
  cardSlotSingle: { flex: 0, width: "50%" },
  cardGap: { marginLeft: 12 },
});
