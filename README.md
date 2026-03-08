# 配置定义的消息转发网络

## 安装

```
go build -ldflags "-s -w" -o ./webhook ./cmd/webhook/
```

## 使用

```
$ ./webhook gen webhook.yaml
2025/10/29 22:25:08 配置文件已生成: webhook.yaml

$ ./webhook view webhook.yaml
=== Webhook 路由配置视图 ===

监听地址: :8080

路由端点:
  /complex -> root
  /broadcast -> broadcast-all
  /broadcast-logged -> broadcast-with-log
  /wecom -> wecom-alerts
  /tradingview/public -> broadcast-all
  /alert/normal -> log-normal
  /alert/important -> log-important
  /alert/urgent -> log-urgent
  /dev/null -> blackhole

节点拓扑图:

└─ root [虚拟节点]
  ├─ node-a [虚拟节点]
  │ └─ node-c [虚拟节点]
  │   └─ telegram-channel [Telegram: 通知频道]
  └─ node-b [虚拟节点]
    └─ wecom-alerts [企业微信: 告警群]
└─ broadcast-all [虚拟节点]
  ├─ wecom-alerts [企业微信: 告警群]
  ├─ telegram-channel [Telegram: 通知频道]
  └─ custom-webhook [Webhook: https://example.com/webhook]
└─ broadcast-with-log [虚拟节点]
  ├─ log-important [日志: 重要]
  │ └─ wecom-alerts [企业微信: 告警群]
  └─ custom-webhook [Webhook: https://example.com/webhook]
└─ log-normal [日志: 一般]
└─ log-urgent [日志: 紧急]
  ├─ wecom-alerts [企业微信: 告警群]
  └─ telegram-channel [Telegram: 通知频道]
└─ blackhole [叶子节点]
```

另一个例子：

```
监听地址: 127.0.0.1:9999

路由端点:
  /tradingview/public -> tv-public
  /private -> private
  /wecom -> wecom-bighand
  /trump -> node-trump

节点拓扑图:

└─ tv-public [虚拟节点]
  ├─ log-normal [日志: 一般]
  └─ wecom-bighand [企业微信: 告警群]
└─ private [虚拟节点]
  ├─ log-important [日志: 重要]
  └─ tg-weaming [Telegram: 私人通知频道]
└─ node-trump [虚拟节点]
  ├─ log-important [日志: 重要]
  ├─ wecom-bighand [企业微信: 告警群]
  └─ tg-trump [Telegram: 川普刚刚说了什么]
```

## 消息格式化 (Telegram)

支持通过以下两种方式指定 Telegram 消息的解析模式：

### 1. 节点配置
在 `nodes` 定义中设置 `parse_mode`：
- `HTML`: 使用 HTML 标签
- `Markdown`: 使用旧版 Markdown (建议用 V2)
- `MarkdownV2`: 使用 MarkdownV2 格式

### 2. Content-Type 自动识别 (优先级最高)
当客户端请求时，服务端会根据请求头中的 `Content-Type` 自动覆盖节点的配置：
- `text/html` → 强制使用 `HTML`
- `text/markdown` → 强制使用 `Markdown`
- `text/x-markdown-v2` → 强制使用 `MarkdownV2`

> [!TIP]
> 代码会自动处理大小写，因此配置 `html` 或 `HTML` 均可生效。