// Command cron 은 출발 lead분 전 우산 푸시를 한 틱 실행한다. spec §5.
//
// 두 모드로 동작한다(코드 동일, cronjob.Run 재사용):
//   - Lambda: EventBridge Scheduler 가 universal target 으로 이 함수를 직접 invoke.
//     AWS_LAMBDA_RUNTIME_API 가 있으면 lambda.Start 로 핸들러 등록.
//   - 로컬:   AWS 환경이 아니면 1회 실행 후 종료(개발/테스트). go run ./cmd/cron.
//
// 테스트 훅(운영 미설정): CRON_NOW="2006-01-02 15:04", CRON_FORCE_SEND=1.
package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/twkim8548/grab-umbrella/server/internal/cronjob"
	"github.com/twkim8548/grab-umbrella/server/internal/push"
	"github.com/twkim8548/grab-umbrella/server/internal/store"
	"github.com/twkim8548/grab-umbrella/server/internal/weather"
)

var kst = time.FixedZone("KST", 9*60*60)

func main() {
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		lambda.Start(handler) // Lambda: Scheduler invoke 마다 handler 실행
		return
	}
	// 로컬: 1회 실행 후 종료.
	if _, err := handler(context.Background()); err != nil {
		log.Fatalf("cron: %v", err)
	}
}

// handler 는 한 틱을 실행한다. Lambda invoke 이벤트는 사용하지 않으므로 인자가 없다
// (EventBridge Scheduler 는 고정 페이로드만 보내고 본문 내용은 불필요).
func handler(ctx context.Context) (cronjob.Result, error) {
	st, err := store.New(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		return cronjob.Result{}, err
	}
	defer st.Close()

	// API와 cron Lambda의 배포 순서는 보장되지 않는다. cron도 실행 전에
	// idempotent migration을 적용해 새 컬럼을 참조하는 코드가 먼저 떠도 안전하게 한다.
	if err := st.Migrate(ctx); err != nil {
		return cronjob.Result{}, err
	}

	wc := weather.New(mustEnv("KMA_SERVICE_KEY"), env("KMA_BASE_URL",
		"http://apis.data.go.kr/1360000/VilageFcstInfoService_2.0"))
	pc := push.New(env("EXPO_PUSH_URL", "https://exp.host/--/api/v2/push/send"))

	return cronjob.Run(ctx,
		cronjob.Deps{Store: st, Weather: wc, Push: pc},
		cronjob.Options{
			Now:       nowKST(),
			LeadMin:   envInt("PUSH_LEAD_MINUTES", 30),
			ForceSend: os.Getenv("CRON_FORCE_SEND") == "1",
		})
}

// nowKST 는 기준 시각(KST). 테스트용 CRON_NOW("2006-01-02 15:04") 설정 시 그 시각(운영 미설정).
func nowKST() time.Time {
	if v := os.Getenv("CRON_NOW"); v != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04", v, kst)
		if err != nil {
			log.Fatalf("cron: invalid CRON_NOW=%q (want \"2006-01-02 15:04\"): %v", v, err)
		}
		log.Printf("cron: ⚠ CRON_NOW override active → %s", t.Format(time.RFC3339))
		return t
	}
	return time.Now().In(kst)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("cron: invalid %s=%q, using default %d", key, v, def)
		return def
	}
	return n
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env: %s", key)
	}
	return v
}
