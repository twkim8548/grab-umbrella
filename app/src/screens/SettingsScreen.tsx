import { View, Text, Pressable, StyleSheet } from "react-native";

// 설정 화면: 집/회사 위치, 출퇴근 시각, 알림 on/off. spec §7.1.
// 저장 시 saveSettings(local) → sync(서버) 단방향. (spec §2)
export default function SettingsScreen({ onClose }: { onClose: () => void }) {
  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Pressable onPress={onClose} hitSlop={12}>
          <Text style={styles.back}>‹ 뒤로</Text>
        </Pressable>
        <Text style={styles.title}>설정</Text>
        <View style={{ width: 48 }} />
      </View>

      {/* TODO: 위치 검색(카카오 지오코딩, spec §8), 시각 피커, 알림 토글 */}
      <Text style={styles.placeholder}>집 / 회사 위치</Text>
      <Text style={styles.placeholder}>출근 / 퇴근 시각</Text>
      <Text style={styles.placeholder}>알림 on/off</Text>
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
  back: { fontSize: 17, color: "#007AFF" },
  title: { fontSize: 17, fontWeight: "600" },
  placeholder: {
    fontSize: 17,
    paddingVertical: 16,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: "#C6C6C8",
  },
});
