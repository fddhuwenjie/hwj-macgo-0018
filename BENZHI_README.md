# 本质评测环境说明

## 项目

- 项目编号：`hwj-macgo-0018`
- 项目名称：幂等请求凭据核心
- 项目说明：管理离线调用幂等键、执行代次、凭据和重放结果，保证重复请求与超时接管的一致性。

## 固定环境

- Go toolchain：`go1.26.5`
- go.mod language version：`go 1.21`
- GOTOOLCHAIN：`local`
- 支持平台：`linux/amd64`、`linux/arm64`
- Docker 基础镜像：`golang:1.26.5-bookworm`
- Docker manifest：`golang@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd`

## 构建

```bash
./build_benzhi_docker.sh hwj-macgo-0018:benzhi-amd64 linux/amd64
./build_benzhi_docker.sh hwj-macgo-0018:benzhi-arm64 linux/arm64
```

## 运行

```bash
docker run --rm -it --network none hwj-macgo-0018:benzhi-amd64 bash
```

## 容器内验证

```bash
go version
go env GOTOOLCHAIN GOPROXY GOMODCACHE GOCACHE
go test ./...
go vet ./...
go build ./...
```
