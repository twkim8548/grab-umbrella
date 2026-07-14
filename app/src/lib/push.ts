// Expo 푸시 토큰 발급. spec §1: 식별자는 Expo 푸시 토큰 (로그인 없음).
import * as Notifications from "expo-notifications";
import * as Application from "expo-application";
import Constants from "expo-constants";
import { Platform } from "react-native";

// 권한 "요청"과 토큰 "발급"을 분리한다(priming 흐름을 깔끔하게).
//  - ensureNotificationPermission(): 필요 시 권한을 요청하고 granted 여부 반환.
//  - getPushToken(): 권한과 무관하게 식별 토큰(정식 또는 dev 폴백)을 항상 반환.
// 메인 화면은 forecast 호출을 위해 토큰이 항상 필요하므로 토큰 발급은 권한과 분리한다.

// 현재 권한 상태. priming 표시 판단(undetermined)에 사용.
export async function getNotificationPermissionStatus(): Promise<Notifications.PermissionStatus> {
  try {
    const { status } = await Notifications.getPermissionsAsync();
    return status;
  } catch {
    // 무료 개인 서명처럼 알림 capability 를 쓸 수 없는 빌드에서도 앱 진입은 막지 않는다.
    return "denied" as Notifications.PermissionStatus;
  }
}

// 권한을 보장(필요 시 요청)하고 granted 여부 반환.
// 이미 granted 면 요청 없이 true. denied 등에서 requestPermissionsAsync 는 iOS 에서
// 시스템 팝업을 다시 띄우지 않으므로(이미 결정됨) 그대로 현재 상태를 반영한다.
export async function ensureNotificationPermission(): Promise<boolean> {
  try {
    const { status: existing } = await Notifications.getPermissionsAsync();
    if (existing === "granted") return true;
    const req = await Notifications.requestPermissionsAsync();
    return req.status === "granted";
  } catch {
    // 알림 설정 실패는 예보/설정 동기화를 막는 치명 오류가 아니다.
    return false;
  }
}

// 식별 토큰을 반환. 권한 거부와 무관하게 항상 토큰을 돌려준다(메인 동작 보장).
// 정식 Expo 푸시 토큰은 권한 granted + EAS projectId 가 필요하다(SDK 49+).
// 둘 중 하나라도 없으면 디바이스 식별자 기반 dev 폴백 토큰을 쓴다.
export async function getPushToken(): Promise<string> {
  const projectId =
    Constants.expoConfig?.extra?.eas?.projectId ??
    Constants.easConfig?.projectId;

  if (projectId) {
    try {
      const { status } = await Notifications.getPermissionsAsync();
      if (status === "granted") {
        const token = await Notifications.getExpoPushTokenAsync({ projectId });
        return token.data;
      }
    } catch {
      // 권한 조회나 정식 토큰 발급 실패 시 아래 dev 폴백으로.
    }
  }

  // 개발용/권한없음 폴백: 디바이스 고유 식별자를 임시 토큰으로.
  const deviceId = await getDeviceId();
  return `dev-${deviceId}`;
}

// 기존 호출부 호환용. 권한을 요청하고, granted 여부와 무관하게 토큰을 반환한다.
// (과거 시그니처는 거부 시 null 이었으나, 토큰은 항상 필요하므로 dev 폴백을 보장한다.)
export async function registerForPushToken(): Promise<string> {
  await ensureNotificationPermission();
  return getPushToken();
}

// 디바이스 고유 식별자. iOS는 idForVendor, Android는 androidId.
async function getDeviceId(): Promise<string> {
  try {
    if (Platform.OS === "ios") {
      const id = await Application.getIosIdForVendorAsync();
      if (id) return id;
    } else if (Platform.OS === "android") {
      const id = Application.getAndroidId();
      if (id) return id;
    }
  } catch {
    // 폴백 아래로.
  }
  // 최후 폴백: 설치 세션 id (없으면 고정값).
  return Constants.sessionId ?? "unknown-device";
}
