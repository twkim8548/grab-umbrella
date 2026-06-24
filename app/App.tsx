import { useEffect, useState } from "react";
import { View, ActivityIndicator, StyleSheet } from "react-native";
import { StatusBar } from "expo-status-bar";
import { SafeAreaProvider, SafeAreaView } from "react-native-safe-area-context";
import { NavigationContainer } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import AsyncStorage from "@react-native-async-storage/async-storage";
import * as Notifications from "expo-notifications";
import MainScreen from "./src/screens/MainScreen";
import SettingsScreen from "./src/screens/SettingsScreen";
import PermissionPrimer from "./src/components/PermissionPrimer";
import {
  ensureNotificationPermission,
  getNotificationPermissionStatus,
} from "./src/lib/push";

// 앱이 포그라운드일 때도 알림을 배너+소리로 보이게 한다(SDK 53+ 기본은 숨김).
// 모듈 로드 시 1회 등록. shouldShowBanner/List 는 SDK 54 필드(구 shouldShowAlert 대체).
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowBanner: true,
    shouldShowList: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
  }),
});

// native-stack: 메인/설정 두 화면. Settings 를 push 하면 iOS edge swipe 뒤로가기와
// 네이티브 전환 애니메이션이 기본 제공된다(UINavigationController 기반).
// 각 화면이 자체 헤더를 그리므로 헤더는 숨긴다.
type RootStackParamList = {
  Main: undefined;
  Settings: undefined;
};
const Stack = createNativeStackNavigator<RootStackParamList>();

// 첫 실행 priming 게이트.
//  - "checking": 권한 상태 확인 중(잠깐 로딩).
//  - "primer":   권한 undetermined + 아직 안내 안 함 → 앱 자체 안내 먼저.
//  - "app":      메인/설정 진입(네비게이션 시작).
type Gate = "checking" | "primer" | "app";

// priming 을 본 적 있는지 기억하는 플래그. "나중에"를 눌렀어도 매번 띄우지 않는다.
const PRIMED_KEY = "grab-umbrella:notif-primed";

export default function App() {
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
    <SafeAreaProvider>
      <StatusBar style="auto" />
      {gate === "checking" ? (
        <SafeAreaView style={styles.container}>
          <View style={styles.center}>
            <ActivityIndicator />
          </View>
        </SafeAreaView>
      ) : gate === "primer" ? (
        <SafeAreaView style={styles.container}>
          <PermissionPrimer onAllow={handleAllow} onSkip={finishPrimer} />
        </SafeAreaView>
      ) : (
        <NavigationContainer>
          <Stack.Navigator screenOptions={{ headerShown: false }}>
            <Stack.Screen name="Main">
              {({ navigation }) => (
                <SafeAreaView style={styles.container} edges={["top", "left", "right"]}>
                  <MainScreen onOpenSettings={() => navigation.navigate("Settings")} />
                </SafeAreaView>
              )}
            </Stack.Screen>
            <Stack.Screen name="Settings">
              {({ navigation }) => (
                <SafeAreaView style={styles.container} edges={["top", "left", "right"]}>
                  <SettingsScreen onClose={() => navigation.goBack()} />
                </SafeAreaView>
              )}
            </Stack.Screen>
          </Stack.Navigator>
        </NavigationContainer>
      )}
    </SafeAreaProvider>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#fff" },
  center: { flex: 1, alignItems: "center", justifyContent: "center" },
});
