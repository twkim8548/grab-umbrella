// 설정의 주인 = 앱 로컬 저장소 (spec §2). 오프라인에서도 보이고, 앱 켜자마자 표시.
import AsyncStorage from "@react-native-async-storage/async-storage";
import type { Settings } from "../lib/types";

const KEY = "grab-umbrella:settings";

export async function loadSettings(): Promise<Settings | null> {
  const raw = await AsyncStorage.getItem(KEY);
  return raw ? (JSON.parse(raw) as Settings) : null;
}

export async function saveSettings(s: Settings): Promise<void> {
  await AsyncStorage.setItem(KEY, JSON.stringify(s));
}
