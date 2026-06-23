import { View, Text, Pressable, StyleSheet } from "react-native";

// 알림 권한 priming (Apple HIG 방식).
// iOS 는 권한 거부 시 재요청이 불가하므로, 시스템 권한 팝업 전에 앱 자체 안내를 먼저 보여준다.
// 권한 상태가 'undetermined' 일 때만 App 레벨에서 1회 표시한다(표시 판단은 호출부 책임).
//  - [알림 허용] → onAllow(): 그때 실제 Notifications.requestPermissionsAsync 호출.
//  - [나중에]   → onSkip(): 권한 없이 진행(앱은 dev 토큰으로 동작).
export default function PermissionPrimer({
  onAllow,
  onSkip,
}: {
  onAllow: () => void;
  onSkip: () => void;
}) {
  return (
    <View style={styles.container}>
      <View style={styles.content}>
        <Text style={styles.icon}>☔️</Text>
        <Text style={styles.title}>출발 전에 우산 챙기라고 알려드릴게요</Text>
        <Text style={styles.body}>
          출근·퇴근길 비 소식을 알림으로 보내드려요.{"\n"}알림을 허용해주세요.
        </Text>
      </View>

      <View style={styles.actions}>
        <Pressable style={styles.primaryButton} onPress={onAllow}>
          <Text style={styles.primaryButtonText}>알림 허용</Text>
        </Pressable>
        <Pressable style={styles.secondaryButton} onPress={onSkip} hitSlop={8}>
          <Text style={styles.secondaryButtonText}>나중에</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    paddingHorizontal: 28,
    paddingBottom: 24,
    justifyContent: "space-between",
  },
  content: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: 16,
  },
  icon: { fontSize: 72 },
  title: {
    fontSize: 28,
    fontWeight: "700",
    color: "#000",
    textAlign: "center",
    lineHeight: 36,
  },
  body: {
    fontSize: 17,
    color: "#8E8E93",
    textAlign: "center",
    lineHeight: 24,
  },
  actions: { gap: 8 },
  primaryButton: {
    backgroundColor: "#007AFF",
    borderRadius: 12,
    paddingVertical: 16,
    alignItems: "center",
  },
  primaryButtonText: { color: "#fff", fontSize: 17, fontWeight: "600" },
  secondaryButton: {
    paddingVertical: 14,
    alignItems: "center",
  },
  secondaryButtonText: { color: "#007AFF", fontSize: 17, fontWeight: "400" },
});
