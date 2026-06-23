// Command cron 은 Render Cron Job 으로 매 N분 깨어나 출발 30분 전 푸시를 발송한다. spec §5.
//
// 웹 서비스와 별도 서비스로 돌아 spin-down 영향을 받지 않는다.
//
// 흐름 (spec §5):
//  1. "지금부터 ~lead분 뒤가 출근/퇴근"인 기기만 DB에서 SELECT
//  2. 각 기기 위치·시각으로 초단기예보 조회 (출근=집, 퇴근=회사)
//  3. 한 줄 메시지로 압축 → Expo Push 발송
//  4. 중복 방지: last_morning_push_date / last_evening_push_date 기록
package main

import (
	"context"
	"log"
	"os"
)

func main() {
	_ = context.Background()
	log.Println("cron: tick start")

	// TODO:
	//   st := store.New(...); wc := weather.New(...); pc := push.New(...)
	//   due := st.DueDevices(ctx, leadMinutes)
	//   for d := range due { fc := wc.UltraSrtForecast(...); pc.Send(...); st.MarkPushed(...) }

	_ = os.Getenv("PUSH_LEAD_MINUTES")
	log.Println("cron: tick done (not implemented)")
}
