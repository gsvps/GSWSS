

> A lightweight transport protocol designed for Cloudflare Workers.

## 项目介绍

GS Protocol 是一套专门针对 Cloudflare Workers 设计的轻量级网络传输协议。

项目目标：

* 无需 VPS
* 用户部署到自己的 Cloudflare Workers
* 支持 TCP 转发
* 支持 SOCKS5
* 支持 HTTP Proxy
* 支持多路复用（Mux）
* 支持 TLS（WSS）
* 支持身份认证
* 支持跨平台客户端

项目定位：

**安全传输协议（Secure Transport Protocol）**

不是公共 VPN，不提供任何公共节点。

---

# 设计目标

## 第一阶段（MVP）

支持：

* SOCKS5 Proxy
* HTTP Proxy
* WebSocket Transport
* Password Authentication
* TCP Relay
* Cloudflare Workers

无需：

* UDP
* QUIC
* WireGuard
* VLESS
* Trojan 兼容

---

# 系统架构

```text
                Browser
                   │
             SOCKS5 / HTTP
                   │
            GS Client (Go)
                   │
          WSS (TLS + Protocol)
                   │
          Cloudflare Worker
                   │
          TCP Socket(connect)
                   │
              Internet
```

---

# 技术栈

## Client

语言：

Go 1.24+

依赖：

```text
net
context
tls
crypto
encoding/binary
gorilla/websocket
cobra
viper
zap
```

GUI：

第一版：

CLI

以后：

Wails

---

## Worker

TypeScript

Cloudflare Workers

Hono

使用：

```text
WebSocket

connect()

KV

D1（以后）

Queues（以后）
```

---

## 管理后台（以后）

Next.js

Tailwind

Shadcn/ui

D1

---

# 仓库结构

```
gs-protocol/

├── docs/
│
├── protocol/
│
├── worker/
│
├── client/
│
├── sdk/
│
├── examples/
│
├── benchmark/
│
└── README.md
```

---

# Client 目录

```
client/

cmd/

internal/

proxy/

socks/

http/

protocol/

mux/

transport/

config/

crypto/

log/

version/
```

---

# Worker

```
worker/

src/

handler/

protocol/

auth/

relay/

config/

utils/
```

---

# MVP 功能

## SOCKS5

支持：

```
CONNECT
```

不支持：

```
UDP ASSOCIATE

BIND
```

---

## HTTP Proxy

支持：

```
CONNECT

GET

POST
```

---

## Worker

支持：

```
WebSocket

↓

Authentication

↓

Target Address

↓

connect()

↓

Pipe
```

---

# 协议设计

Magic

```
0x47535031
```

即：

```
GSP1
```

Version

```
1
```

Frame

```
Magic

Version

Type

Flags

Length

Payload
```

---

# Type

```
CONNECT

DATA

PING

PONG

CLOSE

ERROR
```

---

# CONNECT

Payload

```
Target Host

Target Port

Password
```

---

# DATA

```
Binary
```

---

# Authentication

第一版：

```
Password
```

以后：

```
Token

JWT

OAuth
```

---

# Encryption

依赖：

TLS

即可。

以后：

增加：

```
AES-GCM

ChaCha20
```

二次加密。

---

# Multiplex

以后支持：

```
StreamID
```

一个：

WebSocket

多个：

TCP。

---

# Compression

以后：

```
zstd
```

---

# Heartbeat

```
30s

PING

↓

PONG
```

超时：

```
Disconnect
```

---

# 重连

自动：

```
Reconnect
```

保持：

SOCKS

不断。

---

# 配置

config.yaml

```yaml
server: https://xxxx.workers.dev

password: xxxxxxxxx

local_socks: 127.0.0.1:1080

local_http: 127.0.0.1:7890

tls: true

mux: true

heartbeat: 30
```

---

# Worker 配置

```
PASSWORD
```

以后：

KV：

```
ALLOW_IP

LIMIT

TOKEN
```

---

# CLI

```
gs start

gs stop

gs status

gs version

gs benchmark
```

---

# Benchmark

测速：

```
Latency

Bandwidth

Reconnect

Packet Loss
```

---

# 日志

支持：

```
Debug

Info

Warn

Error
```

---

# 后续路线图

V1

✅ SOCKS

✅ HTTP

✅ Worker

---

V2

✅ Mux

✅ GUI

✅ D1

---

V3

✅ Android

✅ macOS

✅ Linux

---

V4

✅ UDP

✅ QUIC

✅ AI Gateway

---

# 开源协议

MIT

---

# GitHub Topics

```
cloudflare

workers

proxy

socks5

http-proxy

websocket

transport

protocol

edge

golang

typescript

network
```

---

# Cursor 开发规范（必须遵守）

## 代码规范

* Go 代码遵循 Go 官方 Code Review Comments 与 `gofmt`/`goimports`。
* Worker 使用 TypeScript `strict` 模式，禁止 `any`（必要时需注明原因）。
* 所有公共 API 必须有注释。
* 所有网络错误必须返回明确错误码。
* 不允许 panic 作为正常错误处理。

## 架构原则

* 协议层、传输层、代理层完全解耦。
* Worker 不依赖客户端实现细节。
* 客户端可以替换为其他语言实现，只需遵循协议规范。
* 所有协议字段必须向前兼容。

## 性能目标

* 单 Worker 支持数百个并发 TCP Relay（受 Cloudflare 平台限制）。
* 内存占用尽可能低。
* WebSocket 二进制传输，不使用 Base64。
* 零拷贝（Zero-copy）优先。

## 安全要求

* 所有认证信息不得写入日志。
* 默认启用 TLS（WSS）。
* 增加连接速率限制，防止暴力破解。
* 对目标地址进行基本校验，避免明显异常输入。
* 不实现任何绕过认证的调试后门。
