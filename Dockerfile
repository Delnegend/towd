FROM golang:alpine AS server-builder

WORKDIR /app
COPY ./src-server ./src-server
COPY ./main.go ./main.go
COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum

RUN go mod download
RUN go build -o /app/main .

FROM oven/bun:alpine AS client-builder

WORKDIR /app
COPY ./src ./src
COPY ./package.json ./package.json
COPY ./tailwind.config.js ./tailwind.config.js
COPY ./components.json ./components.json
COPY ./tsconfig.json ./tsconfig.json
COPY ./bun.lockb ./bun.lockb
COPY ./nuxt.config.ts ./nuxt.config.ts
RUN bun i --frozen-lockfile && bun generate

FROM alpine:latest

WORKDIR /app
ENV STATIC_WEB_CLIENT_DIR=/app/public
COPY --from=server-builder /app/main /app/main
COPY --from=client-builder /app/.output/public /app/public

EXPOSE 8080

CMD ["/app/main"]