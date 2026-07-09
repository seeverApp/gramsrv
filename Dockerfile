FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-w -s" -o out ./cmd/telesrv

FROM alpine:latest
RUN apk --no-cache add ca-certificates openssl
WORKDIR /root/
RUN mkdir -p /root/data

COPY --from=builder /app/out .

# اسکریپت ورودی برای چاپ کلید عمومی پس از تولید
RUN echo '#!/bin/sh' > entrypoint.sh && \
    echo './out &' >> entrypoint.sh && \
    echo 'PID=$!' >> entrypoint.sh && \
    echo 'while [ ! -f /root/data/server_rsa.pem ]; do sleep 1; done' >> entrypoint.sh && \
    echo 'echo ""' >> entrypoint.sh && \
    echo 'echo "===== RSA PUBLIC KEY (برای اتصال کلاینت) ====="' >> entrypoint.sh && \
    echo 'openssl rsa -in /root/data/server_rsa.pem -RSAPublicKey_out' >> entrypoint.sh && \
    echo 'echo "================================================"' >> entrypoint.sh && \
    echo 'echo ""' >> entrypoint.sh && \
    echo 'wait $PID' >> entrypoint.sh && \
    chmod +x entrypoint.sh

CMD ["./entrypoint.sh"]
