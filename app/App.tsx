import { useEffect, useState } from "react";
import { SafeAreaView, View, ActivityIndicator, StyleSheet } from "react-native";
import { StatusBar } from "expo-status-bar";
import AsyncStorage from "@react-native-async-storage/async-storage";
import MainScreen from "./src/screens/MainScreen";
import SettingsScreen from "./src/screens/SettingsScreen";
import PermissionPrimer from "./src/components/PermissionPrimer";
import {
  ensureNotificationPermission,
  getNotificationPermissionStatus,
} from "./src/lib/push";

// 화면 전환은 우선 단순 상태로. 화면이 늘면 react-navigation 도입 (spec §7.1: 메인/설정).
type Route = "main" | "settings";

// 첫 실행 priming 게이트.
//  - "checking": 권한 상태 확인 중(잠깐 로딩).
//  - "primer":   권한 undetermined + 아직 안내 안 함 → 앱 자체 안내 먼저.
//  - "app":      메인/설정 진입.
type Gate = "checking" | "primer" | "app";

// priming 을 본 적 있는지 기억하는 플래그. "나중에"를 눌렀어도 매번 띄우지 않는다.
const PRIMED_KEY = "grab-umbrella:notif-primed";

export default function App() {
  const [route, setRoute] = useState<Route>("main");
  const [gate, setGate] = useState<Gate>("checking");

  // 앱 시작 시 권한 상태 확인 → undetermined 이고 아직 안 물어봤으면 priming 먼저.
  useEffect(() => {
    (async () => {
      try {
        const status = await getNotificationPermissionStatus();
        if (status === "undetermined") {
          const primed = await AsyncStorage.getItem(PRIMED_KEY);
          setGate(primed ? "app" : "primer");
          return;
        }
        // 이미 granted/denied 면 건너뛴다.
        setGate("app");
      } catch {
        setGate("app");
      }
    })();
  }, []);

  // priming 의 [알림 허용]/[나중에] 공통 후처리: 플래그 기록 후 앱 진입.
  const finishPrimer = async () => {
    try {
      await AsyncStorage.setItem(PRIMED_KEY, "1");
    } catch {
      // 플래그 저장 실패해도 진행.
    }
    setGate("app");
  };

  const handleAllow = async () => {
    // [알림 허용] 누른 그 시점에 실제 시스템 권한 요청.
    await ensureNotificationPermission();
    await finishPrimer();
  };

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar style="auto" />
      {gate === "checking" ? (
        <View style={styles.center}>
          <ActivityIndicator />
        </View>
      ) : gate === "primer" ? (
        <PermissionPrimer onAllow={handleAllow} onSkip={finishPrimer} />
      ) : route === "main" ? (
        <MainScreen onOpenSettings={() => setRoute("settings")} />
      ) : (
        <SettingsScreen onClose={() => setRoute("main")} />
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#fff" },
  center: { flex: 1, alignItems: "center", justifyContent: "center" },
});
