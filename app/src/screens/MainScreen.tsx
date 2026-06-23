import { View, Text, Pressable, StyleSheet } from "react-native";
import CommuteCard from "../components/CommuteCard";

// 메인 화면: 출근 카드 + 퇴근 카드 (두 핵심 시점). spec §7.1.
// TODO: registerForPushToken → getForecast 로 실제 데이터 로드.
export default function MainScreen({ onOpenSettings }: { onOpenSettings: () => void }) {
  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>우산챙겨?</Text>
        <Pressable onPress={onOpenSettings} hitSlop={12}>
          <Text style={styles.gear}>⚙︎</Text>
        </Pressable>
      </View>

      <CommuteCard label="출근" />
      <CommuteCard label="퇴근" />
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
});
