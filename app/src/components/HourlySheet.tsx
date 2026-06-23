import { useEffect, useRef, useState } from "react";
import {
  View,
  Text,
  Pressable,
  Modal,
  ScrollView,
  Animated,
  Easing,
  StyleSheet,
} from "react-native";
import type { HourlyPoint } from "../lib/types";
import { formatHHmm } from "../lib/format";

// 시간별 흐름 하단 시트. 카드 인라인 펼침을 대체한다.
// 추가 라이브러리 없이 RN Modal + Animated 로 구현한다.
// Modal 자체 슬라이드(animationType)는 backdrop 까지 함께 밀어올려 어색하므로,
// Modal 은 즉시 표시하고(animationType none) backdrop 은 fade, 시트만 아래→위로 slide 한다.
export default function HourlySheet({
  visible,
  title,
  hourly,
  onClose,
}: {
  visible: boolean;
  title: string; // "출근 시간대" | "퇴근 시간대"
  hourly: HourlyPoint[] | null;
  onClose: () => void;
}) {
  // 0 = 닫힘(시트 아래로), 1 = 열림. backdrop opacity 와 시트 translateY 를 함께 구동.
  const anim = useRef(new Animated.Value(0)).current;
  // 닫힘 애니메이션이 끝난 뒤에 실제로 언마운트하기 위한 내부 렌더 상태.
  // visible 이 false 가 되면 즉시 Modal 을 내리지 않고, slide-down/fade-out 후 내린다.
  const [rendered, setRendered] = useState(visible);

  useEffect(() => {
    if (visible) {
      setRendered(true);
      Animated.timing(anim, {
        toValue: 1,
        duration: 260,
        easing: Easing.out(Easing.cubic),
        useNativeDriver: true,
      }).start();
    } else if (rendered) {
      // fade-out + slide-down 후 언마운트.
      Animated.timing(anim, {
        toValue: 0,
        duration: 220,
        easing: Easing.in(Easing.cubic),
        useNativeDriver: true,
      }).start(({ finished }) => {
        if (finished) setRendered(false);
      });
    }
  }, [visible, rendered, anim]);

  const translateY = anim.interpolate({
    inputRange: [0, 1],
    outputRange: [600, 0], // 시트를 화면 아래에서 위로.
  });

  return (
    <Modal
      visible={rendered}
      animationType="none"
      transparent
      onRequestClose={onClose}
    >
      {/* 반투명 backdrop: fade 로 깔리고, 탭하면 닫힘 */}
      <Animated.View style={[styles.backdrop, { opacity: anim }]}>
        <Pressable style={StyleSheet.absoluteFill} onPress={onClose} />
      </Animated.View>

      {/* 하단 시트: 시트만 slide(translateY) */}
      <Animated.View style={[styles.sheet, { transform: [{ translateY }] }]}>
        {/* 시각적 grabber */}
        <View style={styles.grabber} />

        <View style={styles.header}>
          <Text style={styles.title}>{title}</Text>
          <Pressable onPress={onClose} hitSlop={12}>
            <Text style={styles.close}>닫기</Text>
          </Pressable>
        </View>

        <ScrollView
          style={styles.list}
          contentContainerStyle={styles.listContent}
        >
          {hourly && hourly.length > 0 ? (
            hourly.map((h) => <HourlyRow key={h.time} point={h} />)
          ) : (
            <Text style={styles.empty}>시간별 정보가 없어요.</Text>
          )}
        </ScrollView>
      </Animated.View>
    </Modal>
  );
}

function HourlyRow({ point }: { point: HourlyPoint }) {
  const isRain = point.ptyText !== "없음";
  return (
    <View style={styles.row}>
      <Text style={styles.time} numberOfLines={1}>
        {formatHHmm(point.time)}
      </Text>
      <Text
        style={[styles.weather, isRain ? styles.weatherRain : styles.weatherDim]}
        numberOfLines={1}
      >
        {isRain ? point.ptyText : "맑음"}
      </Text>
      <Text style={styles.temp} numberOfLines={1}>
        {Math.round(point.tempC)}°
      </Text>
      <Text
        style={[styles.pop, isRain ? styles.popRain : null]}
        numberOfLines={1}
      >
        {point.popPct}%
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  // 화면 전체를 덮는 반투명 영역. 탭하면 닫힌다. 시트는 그 위에 하단 정렬로 얹힌다.
  backdrop: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: "rgba(0,0,0,0.3)",
  },
  // 하단 시트: 내용만큼 높이를 가지되 화면 55% 를 넘지 않는다.
  // 절대 위치로 화면 하단에 고정(backdrop 위).
  sheet: {
    position: "absolute",
    left: 0,
    right: 0,
    bottom: 0,
    maxHeight: "55%",
    backgroundColor: "#fff",
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    paddingHorizontal: 20,
    // iOS 홈 인디케이터 영역 확보(safe-area-context 미사용 환경 대비 고정값).
    paddingBottom: 34,
  },
  grabber: {
    alignSelf: "center",
    width: 36,
    height: 5,
    borderRadius: 2.5,
    backgroundColor: "#C6C6C8",
    marginTop: 8,
    marginBottom: 4,
  },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingVertical: 12,
  },
  title: { fontSize: 22, fontWeight: "700" },
  close: { fontSize: 17, color: "#007AFF" },
  // 내용이 적으면 내용만큼, 많으면 sheet maxHeight 안에서 스크롤.
  list: { flexGrow: 0, flexShrink: 1 },
  listContent: { paddingBottom: 8 },
  empty: {
    fontSize: 15,
    color: "#8E8E93",
    textAlign: "center",
    marginTop: 40,
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    paddingVertical: 14,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: "#C6C6C8",
  },
  time: { fontSize: 16, color: "#000", width: 64 },
  weather: { fontSize: 15, flex: 1, paddingHorizontal: 8 },
  weatherRain: { color: "#007AFF", fontWeight: "600" },
  weatherDim: { color: "#C7C7CC" },
  temp: { fontSize: 16, color: "#000", width: 56, textAlign: "right" },
  pop: { fontSize: 16, color: "#8E8E93", width: 56, textAlign: "right" },
  popRain: { color: "#007AFF", fontWeight: "600" },
});
