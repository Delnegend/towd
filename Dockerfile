FROM golang:alpine AS server-builder

WORKDIR /app
COPY ./src-server ./src-server
COPY ./main.go ./main.go
COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum

RUN go mod download
RUN go build -o /app/main .

FROM node:22-alpine AS client-builder
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

WORKDIR /app
COPY ./src ./src
COPY ./package.json ./package.json
COPY ./tailwind.config.js ./tailwind.config.js
COPY ./components.json ./components.json
COPY ./tsconfig.json ./tsconfig.json
COPY pnpm-lock.yaml ./pnpm-lock.yaml
COPY ./nuxt.config.ts ./nuxt.config.ts
RUN npm i -g pnpm
RUN pnpm i --frozen-lockfile && pnpm generate

FROM alpine:latest

WORKDIR /app
ENV STATIC_WEB_CLIENT_DIR=/app/public
COPY --from=server-builder /app/main /app/main
COPY --from=client-builder /app/.output/public /app/public

EXPOSE 8080

CMD ["/app/main"]