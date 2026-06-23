import { useEffect, useState } from "react";
import {
  View,
  Text,
  Pressable,
  TextInput,
  Switch,
  Alert,
  ActivityIndicator,
  StyleSheet,
  ScrollView,
} from "react-native";
import AddressSearch from "../components/AddressSearch";
import { loadSettings, saveSettings } from "../storage/settings";
import { ensureNotificationPermission, getPushToken } from "../lib/push";
import { sync } from "../lib/api";
import type { Settings } from "../lib/types";

// 설정 화면: 집/회사 주소, 출퇴근 시각, 알림 on/off. spec §7.1.
// 저장 시 saveSettings(local) → sync(서버) 단방향. (spec §2)
//
// UX 메모: 시각 입력은 datetimepicker 대신 "HHmm" 텍스트 입력 + 검증으로 구현했다.
// (네 자리 숫자, 00~23시 / 00~59분 범위 검증.) 추후 네이티브 휠 피커로 교체 가능.
export default function SettingsScreen({ onClose }: { onClose: () => void }) {
  const [homeAddress, setHomeAddress] = useState("");
  const [workAddress, setWorkAddress] = useState("");
  const [commuteStart, setCommuteStart] = useState("0830");
  const [commuteEnd, setCommuteEnd] = useState("1900");
  const [notificationsEnabled, setNotificationsEnabled] = useState(true);

  const [picker, setPicker] = useState<null | "home" | "work">(null);
  const [saving, setSaving] = useState(false);

  // 진입 시 기존 값 채우기.
  useEffect(() => {
    loadSettings().then((s) => {
      if (!s) return;
      setHomeAddress(s.homeAddress);
      setWorkAddress(s.workAddress);
      setCommuteStart(s.commuteStart);
      setCommuteEnd(s.commuteEnd);
      setNotificationsEnabled(s.notificationsEnabled);
    });
  }, []);

  const onAddressSelected = (addr: string) => {
    if (picker === "home") setHomeAddress(addr);
    else if (picker === "work") setWorkAddress(addr);
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
      commuteStart,
      commuteEnd,
      notificationsEnabled,
    };

    setSaving(true);
    try {
      // 1) 로컬 저장 (설정의 주인은 로컬).
      await saveSettings(settings);

      // 2) 알림을 켰다면 권한을 보장(필요 시 요청)한다. 거부돼도 토큰은 dev 폴백으로 발급돼
      //    서버 동기화는 진행한다(예보는 받되, 실제 푸시 발송은 권한이 있어야 활성화).
      let granted = true;
      if (notificationsEnabled) {
        granted = await ensureNotificationPermission();
      }

      // 3) 토큰 획득(권한과 무관하게 항상 발급) 후 서버 동기화.
      const token = await getPushToken();
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
      Alert.alert("동기화 실패", msg);
    } finally {
      setSaving(false);
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Pressable onPress={onClose} hitSlop={12}>
          <Text style={styles.back}>‹ 뒤로</Text>
        </Pressable>
        <Text style={styles.title}>설정</Text>
        <View style={{ width: 48 }} />
      </View>

      <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">
        {/* 위치 그룹 */}
        <Text style={styles.groupHeader}>위치</Text>
        <View style={styles.group}>
          <AddressRow
            label="집"
            value={homeAddress}
            onPress={() => setPicker("home")}
          />
          <View style={styles.separator} />
          <AddressRow
            label="회사"
            value={workAddress}
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
        <Text style={styles.footnote}>네 자리 24시간 형식 (예: 0830, 1900)</Text>

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
  onPress,
}: {
  label: string;
  value: string;
  onPress: () => void;
}) {
  return (
    <Pressable style={styles.row} onPress={onPress}>
      <Text style={styles.rowLabel}>{label}</Text>
      <Text style={[styles.rowValue, !value && styles.rowPlaceholder]} numberOfLines={1}>
        {value || "주소 검색"}
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
  return (
    <View style={styles.row}>
      <Text style={styles.rowLabel}>{label}</Text>
      <TextInput
        style={styles.timeInput}
        value={value}
        onChangeText={(t) => onChange(t.replace(/[^0-9]/g, "").slice(0, 4))}
        keyboardType="number-pad"
        maxLength={4}
        placeholder="0830"
        placeholderTextColor="#C6C6C8"
      />
    </View>
  );
}

// "HHmm" 검증: 4자리 숫자, 00~23시 / 00~59분.
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
  back: { fontSize: 17, color: "#007AFF" },
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
  timeInput: {
    fontSize: 17,
    color: "#000",
    minWidth: 80,
    textAlign: "right",
  },
  footnote: { fontSize: 13, color: "#8E8E93", marginTop: 6, marginLeft: 4 },
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
