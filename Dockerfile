FROM golang:1.27-rc-trixie

WORKDIR /app

COPY .env .
COPY cmd/server ./cmd/server
COPY internal/teams ./internal/teams
COPY internal/webhook ./internal/webhook
COPY go.mod go.sum ./
COPY Makefile .

RUN go mod tidy

EXPOSE 8080

CMD ["make", "run"]
