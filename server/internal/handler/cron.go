package handler

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/twkim8548/grab-umbrella/server/internal/cronjob"
)

var kst = time.FixedZone("KST", 9*60*60)

// CronTick 는 POST /cron/tick. EventBridge Scheduler 가 주기적으로 호출해 한 틱(출발
// lead분 전 우산 푸시)을 실행한다. 외부 노출 엔드포인트이므로 시크릿 토큰으로 보호한다.
//
// 인증: 헤더 "X-Cron-Secret" 가 CronSecret 과 일치해야 한다. CronSecret 이 비어 있으면
// (미설정) 401 로 거부한다(실수로 무방비 노출 방지).
//
// 테스트 훅(운영 미설정): CRON_NOW("2006-01-02 15:04"), CRON_FORCE_SEND=1.
func (h *Handler) CronTick(w http.ResponseWriter, r *http.Request) {
	if h.CronSecret == "" {
		http.Error(w, "cron disabled", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("X-Cron-Secret") != h.CronSecret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if h.Push == nil {
		http.Error(w, "push not configured", http.StatusInternalServerError)
		return
	}

	res, err := cronjob.Run(r.Context(),
		cronjob.Deps{Store: h.Store, Weather: h.Weather, Push: h.Push},
		cronjob.Options{
			Now:       cronNow(),
			LeadMin:   envInt("PUSH_LEAD_MINUTES", 30),
			ForceSend: os.Getenv("CRON_FORCE_SEND") == "1",
		})
	if err != nil {
		log.Printf("cron/tick: %v", err)
		http.Error(w, "tick failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]int{
		"due": res.Due, "sent": res.Sent, "skipped": res.Skipped, "failed": res.Failed,
	})
}

// cronNow 는 기준 시각. CRON_NOW 설정 시 그 시각(KST), 아니면 zero(cronjob 이 now 로 채움).
func cronNow() time.Time {
	if v := os.Getenv("CRON_NOW"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04", v, kst); err == nil {
			log.Printf("cron/tick: ⚠ CRON_NOW override → %s", t.Format(time.RFC3339))
			return t
		}
		log.Printf("cron/tick: invalid CRON_NOW=%q, ignoring", v)
	}
	return time.Time{}
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
