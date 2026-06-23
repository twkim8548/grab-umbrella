// Expo 푸시 토큰 발급. spec §1: 식별자는 Expo 푸시 토큰 (로그인 없음).
import * as Notifications from "expo-notifications";

// 권한 요청 후 Expo 푸시 토큰을 반환. 실패 시 null.
export async function registerForPushToken(): Promise<string | null> {
  const { status: existing } = await Notifications.getPermissionsAsync();
  let status = existing;
  if (existing !== "granted") {
    const req = await Notifications.requestPermissionsAsync();
    status = req.status;
  }
  if (status !== "granted") return null;

  const token = await Notifications.getExpoPushTokenAsync();
  return token.data;
}
