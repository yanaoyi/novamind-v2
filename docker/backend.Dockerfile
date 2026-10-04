# 后端镜像：Go 静态编译 → 精简运行镜像
# 注意：GO_IMAGE 的版本必须 ≥ backend/go.mod 里声明的 go 版本
ARG GO_IMAGE=golang:1.26-alpine
FROM ${GO_IMAGE} AS build
WORKDIR /src

# 依赖单独缓存一层
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend ./backend
COPY prompts ./prompts
RUN cd backend && CGO_ENABLED=0 go build -trimpath -o /out/novamind-server ./cmd/server \
    && cd backend && CGO_ENABLED=0 go build -trimpath -o /out/novamind-migrate ./cmd/migrate

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/novamind-server /app/novamind-server
COPY --from=build /out/novamind-migrate /app/migrate
COPY backend/migrations /app/migrations
EXPOSE 8080
ENTRYPOINT ["/app/novamind-server"]
