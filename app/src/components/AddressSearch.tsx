import { useEffect, useState } from "react";
import { Modal, View, Text, Pressable, ActivityIndicator, SafeAreaView, StyleSheet } from "react-native";
import { WebView } from "react-native-webview";
import type { WebViewMessageEvent } from "react-native-webview";
import type { ShouldStartLoadRequest } from "react-native-webview/lib/WebViewTypes";

// Daum 우편번호 위젯을 react-native-webview 안에서 직접 띄운다.
// 기존 @actbase/react-daum-postcode 는 우편번호 페이지의 외부 링크를
// webview 내부에 가두지 못해 시스템 사파리로 튕기는 버그가 있어 직접 구현으로 교체.
//
// 선택 결과는 위젯의 oncomplete → window.ReactNativeWebView.postMessage 로 RN 에 전달된다.
// 외부 네비게이션은 onShouldStartLoadWithRequest 로 차단해 webview 탈출을 막는다.

// 표준 Daum 우편번호 위젯 HTML. 인라인으로 로드하고 CDN 스크립트만 외부에서 가져온다.
const POSTCODE_HTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no" />
  <style>
    html, body { margin: 0; padding: 0; height: 100%; }
    #wrap {
      position: absolute;
      top: 0; left: 0; right: 0; bottom: 0;
      width: 100%; height: 100%;
    }
    #wrap iframe { width: 100% !important; height: 100% !important; }
  </style>
</head>
<body>
  <div id="wrap"></div>
  <script>
    function post(obj) {
      if (window.ReactNativeWebView) {
        window.ReactNativeWebView.postMessage(JSON.stringify(obj));
      }
    }
    // 위젯이 두 번 임베드되는 것을 막는다.
    var embedded = false;
    function embedPostcode() {
      if (embedded) return;
      if (typeof daum === 'undefined' || !daum.Postcode) return;
      try {
        new daum.Postcode({
          oncomplete: function (data) {
            post(data);
          },
          width: '100%',
          height: '100%',
        }).embed(document.getElementById('wrap'));
        embedded = true;
      } catch (e) {
        post({__error: 'embed'});
      }
    }
    // parser-blocking 외부 script 는 CDN 이 멈추면 아래 watchdog 자체도 시작되지 않는다.
    // 먼저 감시 타이머를 건 뒤 script 를 동적으로 로드한다.
    var watchdog = setTimeout(function () {
      if (!embedded) post({__error: 'timeout'});
    }, 8000);
    var script = document.createElement('script');
    script.src = 'https://t1.daumcdn.net/mapjsapi/bundle/postcode/prod/postcode.v2.js';
    script.onload = function () {
      embedPostcode();
      if (embedded) clearTimeout(watchdog);
    };
    script.onerror = function () {
      clearTimeout(watchdog);
      post({__error: 'script'});
    };
    document.head.appendChild(script);
  </script>
</body>
</html>`;

// 위젯/스크립트 동작에 필요한 daum·kakao 도메인.
function isPostcodeResource(url: string): boolean {
  return (
    url.includes("daumcdn.net") ||
    url.includes("postcode") ||
    url.includes("daum.net") ||
    url.includes("kakao.com") ||
    url.includes("kakaocdn.net")
  );
}

// onSelected 로 넘기는 주소 묶음.
// - roadAddress: 도로명 주소(지오코딩/서버 전송용).
// - jibunAddress: 지번 주소(동 추출 근거, 표시용으론 저장 안 해도 됨).
// - dong: 동네 표시용. 법정동(bname) 우선, 없으면 시군구(sigungu).
export interface SelectedAddress {
  roadAddress: string;
  jibunAddress: string;
  dong: string;
}

export default function AddressSearch({
  visible,
  onSelected,
  onClose,
}: {
  visible: boolean;
  onSelected: (address: SelectedAddress) => void;
  onClose: () => void;
}) {
  const [loadError, setLoadError] = useState(false);
  const [webviewKey, setWebviewKey] = useState(0);

  useEffect(() => {
    if (visible) setLoadError(false);
  }, [visible]);

  const handleMessage = (event: WebViewMessageEvent) => {
    try {
      const data = JSON.parse(event.nativeEvent.data) as {
        __error?: string;
        roadAddress?: string;
        address?: string;
        jibunAddress?: string;
        autoJibunAddress?: string;
        bname?: string;
        sigungu?: string;
      };
      if (data.__error) {
        setLoadError(true);
        return;
      }
      const roadAddress = data.roadAddress || data.address || "";
      const jibunAddress = data.jibunAddress || data.autoJibunAddress || "";
      // 동네 표시: 법정동(bname, "○○동/읍/면") 우선, 비었으면 시군구(sigungu) 폴백.
      const dong = (data.bname && data.bname.trim()) || (data.sigungu && data.sigungu.trim()) || "";
      if (roadAddress) {
        onSelected({ roadAddress, jibunAddress, dong });
        onClose();
      }
    } catch {
      // 우편번호 위젯이 아닌 메시지는 무시한다.
    }
  };

  // webview 탈출(지도 새창 등 외부 사파리 이동)만 차단하고, 위젯 동작에 필요한 로드는 모두 허용한다.
  // 초기 인라인 HTML 로드(about:blank/data:/빈 URL)와 daum·kakao 리소스는 항상 통과시킨다.
  // 그 외 http(s) 로의 top-level 네비게이션만 막아 사파리 탈출을 방지한다.
  const handleShouldStartLoad = (request: ShouldStartLoadRequest): boolean => {
    const { url, navigationType } = request;
    // 초기 문서 / 인라인 / 데이터 스킴은 항상 허용.
    if (!url || url === "about:blank" || url.startsWith("data:") || url.startsWith("file:")) {
      return true;
    }
    // 우편번호 위젯 리소스는 항상 허용.
    if (isPostcodeResource(url)) return true;
    // 사용자가 명시적으로 링크를 눌러 외부로 나가려는 경우만 차단(사파리 탈출 방지).
    if (navigationType === "click") return false;
    // 그 외 위젯이 내부적으로 일으키는 로드(iframe/리소스 등)는 허용.
    return true;
  };

  return (
    <Modal visible={visible} animationType="slide" onRequestClose={onClose}>
      <SafeAreaView style={styles.container}>
        <View style={styles.header}>
          <Text style={styles.title}>주소 검색</Text>
          <Pressable onPress={onClose} hitSlop={12}>
            <Text style={styles.close}>닫기</Text>
          </Pressable>
        </View>
        {loadError ? (
          <View style={styles.error}>
            <Text style={styles.errorTitle}>주소 검색을 불러오지 못했어요.</Text>
            <Text style={styles.errorBody}>네트워크 연결을 확인한 뒤 다시 시도해주세요.</Text>
            <Pressable
              style={styles.retryButton}
              onPress={() => {
                setLoadError(false);
                setWebviewKey((key) => key + 1);
              }}
            >
              <Text style={styles.retryText}>다시 시도</Text>
            </Pressable>
          </View>
        ) : (
          <WebView
            key={webviewKey}
            style={styles.webview}
            source={{ html: POSTCODE_HTML, baseUrl: "https://postcode.map.daum.net/" }}
            originWhitelist={["*"]}
            javaScriptEnabled
            domStorageEnabled
            setSupportMultipleWindows={false}
            onMessage={handleMessage}
            onShouldStartLoadWithRequest={handleShouldStartLoad}
            onError={() => setLoadError(true)}
            startInLoadingState
            renderLoading={() => (
              <View style={styles.loading}>
                <ActivityIndicator size="large" color="#007AFF" />
              </View>
            )}
          />
        )}
      </SafeAreaView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#fff" },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 20,
    paddingVertical: 14,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: "#C6C6C8",
  },
  title: { fontSize: 17, fontWeight: "600" },
  close: { fontSize: 17, color: "#007AFF" },
  webview: { flex: 1, width: "100%" },
  loading: {
    ...StyleSheet.absoluteFillObject,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "#fff",
  },
  error: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 32,
    gap: 10,
  },
  errorTitle: { fontSize: 18, fontWeight: "600", textAlign: "center" },
  errorBody: { fontSize: 15, color: "#636366", textAlign: "center" },
  retryButton: {
    marginTop: 10,
    borderRadius: 10,
    backgroundColor: "#007AFF",
    paddingHorizontal: 20,
    paddingVertical: 12,
  },
  retryText: { color: "#fff", fontSize: 16, fontWeight: "600" },
});
