// 配置文件定义和加载逻辑
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// WecomConfig 企业微信配置
type WecomConfig struct {
	Name string `yaml:"name,omitempty"`
	Key  string `yaml:"key"`
}

// TelegramConfig Telegram 配置
type TelegramConfig struct {
	Name          string `yaml:"name,omitempty"`
	BotToken      string `yaml:"bot_token"`
	ChatID        string `yaml:"chat_id"`
	EnablePreview bool   `yaml:"enable_preview,omitempty"`
	ParseMode     string `yaml:"parse_mode,omitempty"`
}

// WebhookConfig 普通 Webhook 配置
type WebhookConfig struct {
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

// Severity 消息紧急程度
type Severity string

const (
	SeverityNormal    Severity = "normal"
	SeverityImportant Severity = "important"
	SeverityUrgent    Severity = "urgent"
)

// LogConfig 日志配置,记录消息并根据紧急程度路由
type LogConfig struct {
	Severity Severity `yaml:"severity"`
	Next     []string `yaml:"next,omitempty"`
}

// Node 节点定义,可以是外部目标或内部虚拟节点
type Node struct {
	Wecom    *WecomConfig    `yaml:"wecom,omitempty"`
	Telegram *TelegramConfig `yaml:"telegram,omitempty"`
	Webhook  *WebhookConfig  `yaml:"webhook,omitempty"`
	Log      *LogConfig      `yaml:"log,omitempty"`
	Next     []string        `yaml:"next,omitempty"`
}

// Route 路由配置
type Route struct {
	Path string `yaml:"path"`
	Node string `yaml:"node"`
}

// Config 主配置
type Config struct {
	Listen string          `yaml:"listen"`
	Nodes  map[string]Node `yaml:"nodes"`
	Routes []Route         `yaml:"routes"`
}

// LoadConfig 从文件加载配置
func LoadConfig(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Listen == "" {
		c.Listen = ":8080"
	}

	for name, node := range c.Nodes {
		if err := node.Validate(name, c.Nodes); err != nil {
			return fmt.Errorf("节点 %s 验证失败: %w", name, err)
		}
	}

	for _, route := range c.Routes {
		if route.Path == "" {
			return fmt.Errorf("路由路径不能为空")
		}

		if route.Node == "" {
			return fmt.Errorf("路由 %s 的节点不能为空", route.Path)
		}

		if _, exists := c.Nodes[route.Node]; !exists {
			return fmt.Errorf("路由 %s 引用的节点 %s 不存在", route.Path, route.Node)
		}
	}

	return nil
}

// GetNodeType 获取节点类型
func (n *Node) GetNodeType() string {
	if n.Wecom != nil {
		return "wecom"
	}
	if n.Telegram != nil {
		return "telegram"
	}
	if n.Webhook != nil {
		return "webhook"
	}
	if n.Log != nil {
		return "log"
	}
	if len(n.Next) > 0 {
		return "virtual"
	}
	return "leaf"
}

// Validate 验证节点配置
func (n *Node) Validate(name string, allNodes map[string]Node) error {
	configCount := 0
	if n.Wecom != nil {
		configCount++
	}
	if n.Telegram != nil {
		configCount++
	}
	if n.Webhook != nil {
		configCount++
	}
	if n.Log != nil {
		configCount++
	}

	if configCount > 1 {
		return fmt.Errorf("节点只能配置一种外部目标类型")
	}

	if n.Wecom != nil {
		if n.Wecom.Key == "" {
			return fmt.Errorf("wecom key 不能为空")
		}
		if len(n.Next) > 0 {
			return fmt.Errorf("外部目标节点不能有 next 配置")
		}
	}

	if n.Telegram != nil {
		if n.Telegram.BotToken == "" {
			return fmt.Errorf("telegram bot_token 不能为空")
		}
		if n.Telegram.ChatID == "" {
			return fmt.Errorf("telegram chat_id 不能为空")
		}
		if len(n.Next) > 0 {
			return fmt.Errorf("外部目标节点不能有 next 配置")
		}
	}

	if n.Webhook != nil {
		if n.Webhook.URL == "" {
			return fmt.Errorf("webhook url 不能为空")
		}
		if len(n.Next) > 0 {
			return fmt.Errorf("外部目标节点不能有 next 配置")
		}
	}

	if n.Log != nil {
		validSeverities := map[Severity]bool{
			SeverityNormal:    true,
			SeverityImportant: true,
			SeverityUrgent:    true,
		}
		if !validSeverities[n.Log.Severity] {
			return fmt.Errorf("无效的紧急程度: %s", n.Log.Severity)
		}
		if len(n.Next) > 0 {
			return fmt.Errorf("日志节点不能在 Node 级别配置 next,应在 log.next 中配置")
		}
		for _, nextNode := range n.Log.Next {
			if _, exists := allNodes[nextNode]; !exists {
				return fmt.Errorf("log.next 引用的节点 %s 不存在", nextNode)
			}
		}
	}

	for _, nextNode := range n.Next {
		if _, exists := allNodes[nextNode]; !exists {
			return fmt.Errorf("next 引用的节点 %s 不存在", nextNode)
		}
	}

	return nil
}

// GenerateDefaultConfig 生成默认配置模板
func GenerateDefaultConfig() string {
	return `# Webhook 服务配置文件
listen: ":8080"

# 节点定义（可以是外部目标或内部虚拟节点）
nodes:
  # 外部节点 - 企业微信
  wecom-alerts:
    wecom:
      name: "告警群"
      key: "your-wecom-webhook-key"

  # 外部节点 - Telegram
  telegram-channel:
    telegram:
      name: "通知频道"
      bot_token: "your-telegram-bot-token"
      chat_id: "your-chat-id"
      enable_preview: false  # 是否启用链接预览,默认 false
      parse_mode: ""         # 消息解析模式,支持 HTML, Markdown, MarkdownV2 (大小写不敏感)

  # 外部节点 - 普通 Webhook
  custom-webhook:
    webhook:
      url: "https://example.com/webhook"
      headers:
        Content-Type: "application/json"
        Authorization: "Bearer your-token"

  # 日志节点 - 根据紧急程度分类处理
  # 一般消息: 只记录日志
  log-normal:
    log:
      severity: normal

  # 重要消息: 记录日志 + 发送到企业微信
  log-important:
    log:
      severity: important
      next:
        - wecom-alerts

  # 紧急消息: 记录日志 + 发送到企业微信和 Telegram
  log-urgent:
    log:
      severity: urgent
      next:
        - wecom-alerts
        - telegram-channel

  # 内部虚拟节点 - 示例: root -> a, b; a -> c; b -> wecom; c -> telegram
  root:
    next:
      - node-a
      - node-b

  node-a:
    next:
      - node-c

  node-b:
    next:
      - wecom-alerts

  node-c:
    next:
      - telegram-channel

  # 简单的广播节点
  broadcast-all:
    next:
      - wecom-alerts
      - telegram-channel
      - custom-webhook

  # 带日志的广播节点(重要级别)
  broadcast-with-log:
    next:
      - log-important
      - custom-webhook

  # 叶子节点（没有 next，消息到此被丢弃）
  blackhole:
    {}

# 路由配置
routes:
  # 复杂路由示例
  - path: /complex
    node: root

  # 广播到所有目标
  - path: /broadcast
    node: broadcast-all

  # 广播并记录日志
  - path: /broadcast-logged
    node: broadcast-with-log

  # 直接发送到企业微信
  - path: /wecom
    node: wecom-alerts

  # TradingView webhook
  - path: /tradingview/public
    node: broadcast-all

  # 不同紧急程度的消息
  - path: /alert/normal
    node: log-normal

  - path: /alert/important
    node: log-important

  - path: /alert/urgent
    node: log-urgent

  # 黑洞路由（消息被丢弃）
  - path: /dev/null
    node: blackhole
`
}
