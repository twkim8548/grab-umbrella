# 우산챙겨? Go 백엔드 — AWS Lambda (Web Adapter) 컨테이너 이미지.
# chi HTTP 서버(cmd/api)를 코드 수정 없이 Lambda 에서 구동한다. Web Adapter 가 Lambda
# 호출을 HTTP 요청으로 변환해 :8080 의 우리 서버로 넘긴다.
#
# cron 은 별도 함수가 아니라 api 의 POST /cron/tick 으로 통합(EventBridge Scheduler 가 호출).
# 빌드 컨텍스트는 server/ 디렉토리(go.mod 위치).

# ── build stage ──
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ./cmd/api

# ── run stage ──
# Web Adapter 바이너리를 /opt/extensions 에 둔다(Lambda 가 익스텐션으로 자동 로드).
# 일반 컨테이너처럼 우리 서버(:8080)를 그대로 실행하면, 어댑터가 Lambda 호출을 HTTP 로 변환.
FROM alpine:3.20
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:1.0.1 /lambda-adapter /opt/extensions/lambda-adapter
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/app /app/app

# Web Adapter 환경변수(AWS_LWA_ 접두사): 우리 서버는 8080 리슨, "/healthz" 로 readiness 체크.
# cron 은 별도 Lambda(Dockerfile.cron)라 여기선 HTTP(/sync·/forecast)만 다룬다.
ENV AWS_LWA_PORT=8080
ENV AWS_LWA_READINESS_CHECK_PATH=/healthz
EXPOSE 8080
ENTRYPOINT ["/app/app"]
