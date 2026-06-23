import { useState } from "react";
import { SafeAreaView, StyleSheet } from "react-native";
import { StatusBar } from "expo-status-bar";
import MainScreen from "./src/screens/MainScreen";
import SettingsScreen from "./src/screens/SettingsScreen";

// 화면 전환은 우선 단순 상태로. 화면이 늘면 react-navigation 도입 (spec §7.1: 메인/설정).
type Route = "main" | "settings";

export default function App() {
  const [route, setRoute] = useState<Route>("main");

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar style="auto" />
      {route === "main" ? (
        <MainScreen onOpenSettings={() => setRoute("settings")} />
      ) : (
        <SettingsScreen onClose={() => setRoute("main")} />
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#fff" },
});
