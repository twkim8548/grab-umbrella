import { useEffect, useState } from "react";
import {
  View,
  Text,
  Pressable,
  Switch,
  Alert,
  ActivityIndicator,
  StyleSheet,
  ScrollView,
  Platform,
} from "react-native";
import DateTimePicker, {
  type DateTimePickerEvent,
} from "@react-native-community/datetimepicker";
import { usePreventRemove } from "@react-navigation/native";
import AddressSearch from "../components/AddressSearch";
import type { SelectedAddress } from "../components/AddressSearch";
import { loadSettings, saveSettings } from "../storage/settings";
import { ensureNotificationPermission, getPushToken } from "../lib/push";
import { sync } from "../lib/api";
import type { Settings } from "../lib/types";
import { formatHHmm } from "../lib/format";

// 출근일 표현: 7자리 "일월화수목금토", 1=on. cron/서버와 동일 계약.
const DAY_LABELS = ["일", "월", "화", "수", "목", "금", "토"] as const;
const WEEKDAYS = "0111110"; // 월~금
// 한 요일 토글: i 번째 비트를 뒤집어 새 문자열 반환.
function toggleDay(days: string, i: number): string {
  const arr = days.split("");
  arr[i] = arr[i] === "1" ? "0" : "1";
  return arr.join("");
}
// 7자리 0/1 문자열인지 검증(구버전 로컬값/누락 방어).
function isValidDays(v: unknown): v is string {
  return typeof v === "string" && /^[01]{7}$/.test(v);
}

// 설정 화면: 집/회사 주소, 출퇴근 시각, 출근일, 알림 on/off. spec §7.1.
// 저장 시 saveSettings(local) → sync(서버) 단방향. (spec §2)
//
// UX 메모: 출퇴근 시각은 네이티브 시간 피커(@react-native-community/datetimepicker,
// mode="time")로 입력한다. 행을 탭하면 iOS 는 compact 팝오버, Android 는 모달
// 다이얼로그가 뜬다. 저장/서버 계약은 그대로 "HHmm" 문자열을 유지하므로,
// 피커가 주는 Date 를 dateToHHmm 으로 변환해 상태에 담는다.
export default function SettingsScreen({ onClose }: { onClose: () => void }) {
  const [homeAddress, setHomeAddress] = useState("");
  const [workAddress, setWorkAddress] = useState("");
  const [homeDong, setHomeDong] = useState("");
  const [workDong, setWorkDong] = useState("");
  const [commuteStart, setCommuteStart] = useState("0830");
  const [commuteEnd, setCommuteEnd] = useState("1900");
  const [commuteDays, setCommuteDays] = useState(WEEKDAYS); // 기본 평일(월~금)
  const [notificationsEnabled, setNotificationsEnabled] = useState(true);

  const [picker, setPicker] = useState<null | "home" | "work">(null);
  const [saving, setSaving] = useState(false);

  // 저장 중 edge swipe/시스템 뒤로가기로 화면이 먼저 닫히면 Main 이 /sync 완료 전에
  // 조회해 미동기화 상태를 볼 수 있다. 저장이 끝날 때까지만 화면 제거를 막는다.
  usePreventRemove(saving, () => {});

  // 진입 시 기존 값 채우기.
  useEffect(() => {
    loadSettings().then((s) => {
      if (!s) return;
      setHomeAddress(s.homeAddress);
      setWorkAddress(s.workAddress);
      setHomeDong(s.homeDong ?? "");
      setWorkDong(s.workDong ?? "");
      setCommuteStart(s.commuteStart);
      setCommuteEnd(s.commuteEnd);
      setCommuteDays(isValidDays(s.commuteDays) ? s.commuteDays : WEEKDAYS);
      setNotificationsEnabled(s.notificationsEnabled);
    });
  }, []);

  const onAddressSelected = (addr: SelectedAddress) => {
    if (picker === "home") {
      setHomeAddress(addr.roadAddress);
      setHomeDong(addr.dong);
    } else if (picker === "work") {
      setWorkAddress(addr.roadAddress);
      setWorkDong(addr.dong);
    }
    setPicker(null);
  };

  const handleSave = async () => {
    if (!homeAddress.trim() || !workAddress.trim()) {
      Alert.alert("주소 입력", "집과 회사 주소를 모두 입력해주세요.");
      return;
    }
    if (!isValidHHmm(commuteStart) || !isValidHHmm(commuteEnd)) {
      Alert.alert("시각 형식", "시각은 HHmm 형식 (예: 0830) 으로 입력해주세요.");
      return;
    }

    const settings: Settings = {
      homeAddress: homeAddress.trim(),
      workAddress: workAddress.trim(),
      homeDong,
      workDong,
      commuteStart,
      commuteEnd,
      commuteDays,
      notificationsEnabled,
    };

    let localSaved = false;
    setSaving(true);
    try {
      // 1) 로컬 저장 (설정의 주인은 로컬).
      await saveSettings(settings);
      localSaved = true;

      // 2) 알림을 켰다면 권한을 보장(필요 시 요청)한다. 거부돼도 토큰은 dev 폴백으로 발급돼
      //    서버 동기화는 진행한다(예보는 받되, 실제 푸시 발송은 권한이 있어야 활성화).
      let granted = true;
      if (notificationsEnabled) {
        granted = await ensureNotificationPermission();
      }

      // 3) 토큰 획득(권한과 무관하게 항상 발급) 후 서버 동기화.
      // 권한 허용 직후에는 세션에 남아 있던 dev 폴백 대신 정식 Expo 토큰을 다시 발급한다.
      const token = await getPushToken({ refreshDevToken: notificationsEnabled && granted });
      await sync(token, settings);

      if (notificationsEnabled && !granted) {
        Alert.alert(
          "저장됨",
          "설정이 저장되고 동기화됐어요. 다만 알림 권한이 없어 푸시 알림은 받을 수 없어요. 기기 설정에서 알림을 허용해주세요.",
          [{ text: "확인", onPress: onClose }]
        );
        return;
      }
      Alert.alert("저장됨", "설정이 저장되고 동기화되었습니다.", [
        { text: "확인", onPress: onClose },
      ]);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "알 수 없는 오류가 발생했습니다.";
      if (localSaved) {
        Alert.alert(
          "설정은 저장됐어요",
          `휴대폰에는 저장했지만 서버 동기화에 실패했어요. 네트워크를 확인한 뒤 다시 저장해주세요.\n\n${msg}`
        );
      } else {
        Alert.alert("설정 저장 실패", `휴대폰에 설정을 저장하지 못했어요. 다시 시도해주세요.\n\n${msg}`);
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Pressable
          onPress={onClose}
          hitSlop={12}
          style={styles.backButton}
          disabled={saving}
        >
          <Text style={styles.back}>‹</Text>
        </Pressable>
        <Text style={styles.title}>설정</Text>
        <View style={styles.backButton} />
      </View>

      <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">
        {/* 위치 그룹 */}
        <Text style={styles.groupHeader}>위치</Text>
        <View style={styles.group}>
          <AddressRow
            label="집"
            value={homeAddress}
            dong={homeDong}
            onPress={() => setPicker("home")}
          />
          <View style={styles.separator} />
          <AddressRow
            label="회사"
            value={workAddress}
            dong={workDong}
            onPress={() => setPicker("work")}
          />
        </View>

        {/* 시각 그룹 */}
        <Text style={styles.groupHeader}>출퇴근 시각</Text>
        <View style={styles.group}>
          <TimeRow label="출근" value={commuteStart} onChange={setCommuteStart} />
          <View style={styles.separator} />
          <TimeRow label="퇴근" value={commuteEnd} onChange={setCommuteEnd} />
        </View>

        {/* 출근일 그룹 */}
        <Text style={styles.groupHeader}>출근일</Text>
        <View style={styles.group}>
          <View style={styles.daysRow}>
            {DAY_LABELS.map((label, i) => {
              const on = commuteDays[i] === "1";
              return (
                <Pressable
                  key={i}
                  style={[styles.dayToggle, on && styles.dayToggleOn]}
                  onPress={() => setCommuteDays(toggleDay(commuteDays, i))}
                >
                  <Text style={[styles.dayLabel, on && styles.dayLabelOn]}>{label}</Text>
                </Pressable>
              );
            })}
          </View>
          <View style={styles.separator} />
          <Pressable style={styles.row} onPress={() => setCommuteDays(WEEKDAYS)}>
            <Text style={styles.weekdayPreset}>월~금 자동 선택</Text>
          </Pressable>
        </View>
        <Text style={styles.helperText}>
          출근·퇴근 시간 30분 전에 알려드려요. (선택한 출근일에만)
        </Text>

        {/* 알림 그룹 */}
        <Text style={styles.groupHeader}>알림</Text>
        <View style={styles.group}>
          <View style={styles.row}>
            <Text style={styles.rowLabel}>알림 받기</Text>
            <Switch value={notificationsEnabled} onValueChange={setNotificationsEnabled} />
          </View>
        </View>

        <Pressable
          style={[styles.saveButton, saving && styles.saveButtonDisabled]}
          onPress={handleSave}
          disabled={saving}
        >
          {saving ? (
            <ActivityIndicator color="#fff" />
          ) : (
            <Text style={styles.saveButtonText}>저장</Text>
          )}
        </Pressable>
      </ScrollView>

      {/* Daum 우편번호 검색 모달 */}
      <AddressSearch
        visible={picker !== null}
        onSelected={onAddressSelected}
        onClose={() => setPicker(null)}
      />
    </View>
  );
}

function AddressRow({
  label,
  value,
  dong,
  onPress,
}: {
  label: string;
  value: string;
  dong: string;
  onPress: () => void;
}) {
  // dong 이 있으면 "역삼동 · 도로명주소" 식으로 앞에 작게 덧붙인다.
  const display = value ? (dong ? `${dong} · ${value}` : value) : "주소 검색";
  return (
    <Pressable style={styles.row} onPress={onPress}>
      <Text style={styles.rowLabel}>{label}</Text>
      <Text style={[styles.rowValue, !value && styles.rowPlaceholder]} numberOfLines={1}>
        {display}
      </Text>
    </Pressable>
  );
}

function TimeRow({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  // iOS 는 compact 디스플레이를 인라인으로 항상 렌더(행 우측 칩, 탭하면 팝오버).
  // Android 는 모달 다이얼로그이므로 show 상태로 토글한다.
  const [show, setShow] = useState(false);

  const handleChange = (event: DateTimePickerEvent, date?: Date) => {
    // Android: dismiss(취소) 시에도 콜백이 오므로 항상 닫는다.
    if (Platform.OS === "android") setShow(false);
    if (event.type === "set" && date) {
      onChange(dateToHHmm(date));
    }
  };

  if (Platform.OS === "ios") {
    return (
      <View style={styles.row}>
        <Text style={styles.rowLabel}>{label}</Text>
        <DateTimePicker
          value={hhmmToDate(value)}
          mode="time"
          display="compact"
          minuteInterval={5}
          onChange={handleChange}
        />
      </View>
    );
  }

  return (
    <Pressable style={styles.row} onPress={() => setShow(true)}>
      <Text style={styles.rowLabel}>{label}</Text>
      <Text style={styles.rowValue}>{formatHHmm(value)}</Text>
      {show && (
        <DateTimePicker
          value={hhmmToDate(value)}
          mode="time"
          display="default"
          minuteInterval={5}
          onChange={handleChange}
        />
      )}
    </Pressable>
  );
}

// "HHmm" → Date (오늘 날짜에 그 시각). 형식이 깨졌으면 자정으로 폴백.
function hhmmToDate(hhmm: string): Date {
  const d = new Date();
  const hh = Number(hhmm.slice(0, 2));
  const mm = Number(hhmm.slice(2));
  d.setHours(Number.isNaN(hh) ? 0 : hh, Number.isNaN(mm) ? 0 : mm, 0, 0);
  return d;
}

// Date → "HHmm" (zero-pad). 저장/서버 계약 형식.
function dateToHHmm(date: Date): string {
  const hh = String(date.getHours()).padStart(2, "0");
  const mm = String(date.getMinutes()).padStart(2, "0");
  return `${hh}${mm}`;
}

// "HHmm" 검증: 4자리 숫자, 00~23시 / 00~59분.
// 피커 값은 항상 유효하지만, 저장 직전 방어용으로 남겨둔다.
function isValidHHmm(v: string): boolean {
  if (!/^\d{4}$/.test(v)) return false;
  const hh = Number(v.slice(0, 2));
  const mm = Number(v.slice(2));
  return hh >= 0 && hh <= 23 && mm >= 0 && mm <= 59;
}

const styles = StyleSheet.create({
  container: { flex: 1, paddingHorizontal: 20, paddingTop: 8 },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: 16,
  },
  // 뒤로: iOS 스타일 chevron 하나. 좌(아이콘)·우(spacer) 동일 폭으로 제목을 중앙 정렬.
  backButton: { width: 44 },
  back: { fontSize: 30, color: "#007AFF", fontWeight: "400", marginTop: -4 },
  title: { fontSize: 17, fontWeight: "600" },
  scroll: { paddingBottom: 40 },
  groupHeader: {
    fontSize: 13,
    color: "#8E8E93",
    textTransform: "uppercase",
    marginTop: 24,
    marginBottom: 8,
    marginLeft: 4,
  },
  group: {
    backgroundColor: "#F2F2F7",
    borderRadius: 12,
    overflow: "hidden",
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 16,
    paddingVertical: 14,
    minHeight: 52,
  },
  separator: {
    height: StyleSheet.hairlineWidth,
    backgroundColor: "#C6C6C8",
    marginLeft: 16,
  },
  rowLabel: { fontSize: 17, color: "#000" },
  rowValue: { fontSize: 17, color: "#000", flexShrink: 1, marginLeft: 12, textAlign: "right" },
  rowPlaceholder: { color: "#007AFF" },
  // 출근일 토글 행: 7개 요일 칩을 가로로 균등 배치.
  daysRow: {
    flexDirection: "row",
    justifyContent: "space-between",
    paddingHorizontal: 12,
    paddingVertical: 12,
  },
  dayToggle: {
    width: 38,
    height: 38,
    borderRadius: 19,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#E5E5EA",
  },
  dayToggleOn: { backgroundColor: "#007AFF" },
  dayLabel: { fontSize: 15, fontWeight: "600", color: "#8E8E93" },
  dayLabelOn: { color: "#fff" },
  weekdayPreset: { fontSize: 17, color: "#007AFF" },
  helperText: { fontSize: 13, color: "#8E8E93", marginTop: 8, marginLeft: 4 },
  saveButton: {
    backgroundColor: "#007AFF",
    borderRadius: 12,
    paddingVertical: 16,
    alignItems: "center",
    marginTop: 32,
  },
  saveButtonDisabled: { opacity: 0.6 },
  saveButtonText: { color: "#fff", fontSize: 17, fontWeight: "600" },
});
