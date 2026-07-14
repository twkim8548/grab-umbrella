import AsyncStorage from "@react-native-async-storage/async-storage";
import type { ForecastResponse, Settings, SlotForecast } from "../lib/types";

const KEY = "grab-umbrella:forecast-cache";
const CACHE_VERSION = 1;
const CACHE_TTL_MS = 3 * 60 * 60 * 1000;
const KST_OFFSET_MS = 9 * 60 * 60 * 1000;

interface ForecastCache {
  version: typeof CACHE_VERSION;
  fingerprint: string;
  cachedAt: number;
  kstDate: string;
  forecast: ForecastResponse;
}

// 예보 결과를 바꾸는 설정만 포함한다. 알림 여부/요일/표시용 동 이름은 예보 자체와
// 무관하므로 캐시를 불필요하게 무효화하지 않는다.
function settingsFingerprint(settings: Settings): string {
  return JSON.stringify([
    settings.homeAddress,
    settings.workAddress,
    settings.commuteStart,
    settings.commuteEnd,
  ]);
}

function kstDate(nowMs: number): string {
  return new Date(nowMs + KST_OFFSET_MS).toISOString().slice(0, 10);
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function isSlotForecast(value: unknown): value is SlotForecast {
  if (!isObject(value)) return false;
  const hourlyValid =
    value.hourly === undefined ||
    (Array.isArray(value.hourly) &&
      value.hourly.every(
        (point) =>
          isObject(point) &&
          typeof point.time === "string" &&
          isFiniteNumber(point.tempC) &&
          isFiniteNumber(point.popPct) &&
          typeof point.ptyText === "string"
      ));
  return (
    typeof value.skyText === "string" &&
    typeof value.ptyText === "string" &&
    isFiniteNumber(value.tempC) &&
    isFiniteNumber(value.popPct) &&
    typeof value.needUmbrella === "boolean" &&
    typeof value.umbrellaReason === "string" &&
    (value.feelsVsYesterday === undefined || typeof value.feelsVsYesterday === "string") &&
    hourlyValid
  );
}

function isDayForecast(value: unknown): boolean {
  if (!isObject(value)) return false;
  return (
    (value.morning === null || isSlotForecast(value.morning)) &&
    (value.evening === null || isSlotForecast(value.evening))
  );
}

function isForecastResponse(value: unknown): value is ForecastResponse {
  return isObject(value) && isDayForecast(value.today) && isDayForecast(value.tomorrow);
}

function isForecastCache(value: unknown): value is ForecastCache {
  if (!isObject(value)) return false;
  return (
    value.version === CACHE_VERSION &&
    typeof value.fingerprint === "string" &&
    isFiniteNumber(value.cachedAt) &&
    typeof value.kstDate === "string" &&
    isForecastResponse(value.forecast)
  );
}

async function clearForecastCache(): Promise<void> {
  await AsyncStorage.removeItem(KEY).catch(() => undefined);
}

function isPastTodaySlot(nowMs: number, commuteTime: string): boolean {
  if (!/^\d{4}$/.test(commuteTime)) return false;
  const hour = Number(commuteTime.slice(0, 2));
  const minute = Number(commuteTime.slice(2));
  if (hour > 23 || minute > 59) return false;

  const nowKst = new Date(nowMs + KST_OFFSET_MS);
  const nowSeconds =
    nowKst.getUTCHours() * 60 * 60 +
    nowKst.getUTCMinutes() * 60 +
    nowKst.getUTCSeconds();
  // 서버 Forecast가 NormalizeToHour로 분을 버리고 정시 슬롯을 사용하므로 동일한
  // 기준으로 제거해야 08:30 설정의 08:00 카드가 정시를 지난 뒤 다시 나타나지 않는다.
  return nowSeconds > hour * 60 * 60;
}

// 캐시를 받은 뒤 시간이 흘렀다면 오늘의 지난 슬롯만 제거한다. 원본 객체를 변경하지
// 않아 이후 저장이나 다른 화면에서 오래된 슬롯이 다시 섞이지 않게 한다.
function withoutPastTodaySlots(
  forecast: ForecastResponse,
  settings: Settings,
  nowMs: number
): ForecastResponse {
  return {
    today: {
      morning: isPastTodaySlot(nowMs, settings.commuteStart) ? null : forecast.today.morning,
      evening: isPastTodaySlot(nowMs, settings.commuteEnd) ? null : forecast.today.evening,
    },
    tomorrow: forecast.tomorrow,
  };
}

export async function loadForecastCache(
  settings: Settings,
  nowMs = Date.now()
): Promise<ForecastResponse | null> {
  let raw: string | null;
  try {
    raw = await AsyncStorage.getItem(KEY);
  } catch {
    return null;
  }
  if (!raw) return null;

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    await clearForecastCache();
    return null;
  }

  if (!isForecastCache(parsed)) {
    await clearForecastCache();
    return null;
  }
  if (
    parsed.fingerprint !== settingsFingerprint(settings) ||
    parsed.cachedAt > nowMs ||
    nowMs - parsed.cachedAt >= CACHE_TTL_MS ||
    parsed.kstDate !== kstDate(nowMs)
  ) {
    await clearForecastCache();
    return null;
  }

  return withoutPastTodaySlots(parsed.forecast, settings, nowMs);
}

export async function saveForecastCache(
  settings: Settings,
  forecast: ForecastResponse,
  nowMs = Date.now()
): Promise<void> {
  const cache: ForecastCache = {
    version: CACHE_VERSION,
    fingerprint: settingsFingerprint(settings),
    cachedAt: nowMs,
    kstDate: kstDate(nowMs),
    forecast,
  };
  // 캐시는 성능 최적화일 뿐이므로 저장소 오류가 정상 예보 표시를 막아서는 안 된다.
  await AsyncStorage.setItem(KEY, JSON.stringify(cache)).catch(() => undefined);
}
