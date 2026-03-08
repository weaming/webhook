// 消息发送器实现,支持基于图的路由系统
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// MessageRouter 消息路由器
type MessageRouter struct {
	config         *Config
	lastMessageIDs map[string]int64 // chat_id -> last message_id
}

// NewMessageRouter 创建消息路由器
func NewMessageRouter(config *Config) *MessageRouter {
	return &MessageRouter{
		config:         config,
		lastMessageIDs: make(map[string]int64),
	}
}

// SendWithAction 发送消息到指定节点并指定操作类型
func (r *MessageRouter) SendWithAction(nodeName string, message string, contentType string, messageID int64, action TelegramAction) error {
	visited := make(map[string]bool)
	return r.sendToNodeWithAction(nodeName, message, contentType, messageID, action, visited)
}

// sendToNodeWithAction 带操作类型的节点发送
func (r *MessageRouter) sendToNodeWithAction(nodeName string, message string, contentType string, messageID int64, action TelegramAction, visited map[string]bool) error {
	if visited[nodeName] {
		log.Printf("检测到循环引用,跳过节点: %s", nodeName)
		return nil
	}

	visited[nodeName] = true

	node, exists := r.config.Nodes[nodeName]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeName)
	}

	nodeType := node.GetNodeType()
	log.Printf("处理节点 %s (类型: %s)", nodeName, nodeType)

	if node.Wecom != nil {
		return sendToWecom(node.Wecom, message)
	}

	if node.Telegram != nil {
		params := &TelegramMessageParams{
			ChatID:        node.Telegram.ChatID,
			Text:          message,
			ParseMode:     "", // contentType 应该在路由配置中指定，不从 HTTP Content-Type 获取
			EnablePreview: node.Telegram.EnablePreview,
			MessageID:     messageID,
			Action:        action,
		}

		// 对于发送操作，保存最后的消息 ID
		if action == TelegramActionSend {
			msgID, err := sendToTelegram(node.Telegram, params)
			if err != nil {
				return err
			}
			if msgID > 0 {
				r.lastMessageIDs[node.Telegram.ChatID] = msgID
				log.Printf("[%s] 已保存 message_id: %d", node.Telegram.Name, msgID)
			}
			return nil
		}

		// 对于编辑/删除操作，优先使用传入的 messageID，其次使用缓存的
		if params.MessageID == 0 {
			if lastID, exists := r.lastMessageIDs[node.Telegram.ChatID]; exists {
				params.MessageID = lastID
				log.Printf("[%s] 使用缓存的 message_id: %d", node.Telegram.Name, lastID)
			} else {
				return fmt.Errorf("请提供 message_id 参数")
			}
		}

		log.Printf("[%s] 执行 %s 操作，message_id: %d", node.Telegram.Name, action, params.MessageID)

		_, err := sendToTelegram(node.Telegram, params)
		// 删除成功后清除缓存
		if err == nil && action == TelegramActionDelete {
			delete(r.lastMessageIDs, node.Telegram.ChatID)
			log.Printf("[%s] 删除成功，已清除缓存的 message_id", node.Telegram.Name)
		}
		return err
	}

	if node.Webhook != nil {
		return sendToWebhook(node.Webhook, message)
	}

	if node.Log != nil {
		logMessage(node.Log, message)

		if len(node.Log.Next) > 0 {
			hasError := false
			for _, nextNode := range node.Log.Next {
				visitedCopy := make(map[string]bool)
				for k, v := range visited {
					visitedCopy[k] = v
				}

				if err := r.sendToNodeWithAction(nextNode, message, contentType, messageID, action, visitedCopy); err != nil {
					log.Printf("发送到日志下级节点 %s 失败: %v", nextNode, err)
					hasError = true
				}
			}

			if hasError {
				return fmt.Errorf("部分日志下级节点发送失败")
			}
		}
		return nil
	}

	if len(node.Next) > 0 {
		hasError := false
		for _, nextNode := range node.Next {
			visitedCopy := make(map[string]bool)
			for k, v := range visited {
				visitedCopy[k] = v
			}

			if err := r.sendToNodeWithAction(nextNode, message, contentType, messageID, action, visitedCopy); err != nil {
				log.Printf("发送到下级节点 %s 失败: %v", nextNode, err)
				hasError = true
			}
		}

		if hasError {
			return fmt.Errorf("部分下级节点发送失败")
		}
		return nil
	}

	log.Printf("叶子节点 %s: 消息被丢弃", nodeName)
	return nil
}

func sendToWecom(config *WecomConfig, message string) error {
	type wecomMessage struct {
		MsgType string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}

	msg := wecomMessage{MsgType: "text"}
	msg.Text.Content = message

	jsonPayload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("创建 JSON payload 失败: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s", config.Key)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("发送到企业微信失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("企业微信返回错误状态 %s: %s", resp.Status, string(responseBody))
	}

	target := "企业微信"
	if config.Name != "" {
		target = fmt.Sprintf("企业微信[%s]", config.Name)
	}
	log.Printf("%s 消息发送成功", target)
	return nil
}

// TelegramAction Telegram 操作类型
type TelegramAction string

const (
	TelegramActionSend   TelegramAction = "send"
	TelegramActionEdit   TelegramAction = "edit"
	TelegramActionDelete TelegramAction = "delete"
)

// TelegramMessageParams Telegram 消息参数
type TelegramMessageParams struct {
	ChatID        string
	Text          string
	MessageID     int64
	ParseMode     string
	EnablePreview bool
	Action        TelegramAction
}

func sendToTelegram(config *TelegramConfig, params *TelegramMessageParams) (int64, error) {
	var url string
	var jsonPayload []byte
	var err error

	switch params.Action {
	case TelegramActionEdit:
		url = fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", config.BotToken)
		jsonPayload, err = buildEditMessagePayload(params)
	case TelegramActionDelete:
		url = fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", config.BotToken)
		jsonPayload, err = buildDeleteMessagePayload(params)
	default:
		url = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.BotToken)
		jsonPayload, err = buildSendMessagePayload(config, params)
	}

	if err != nil {
		return 0, fmt.Errorf("创建 JSON payload 失败: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return 0, fmt.Errorf("发送到 Telegram 失败: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Telegram 返回错误状态 %s: %s", resp.Status, string(responseBody))
	}

	// 解析响应获取 message_id
	var telegramResp struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}

	if err := json.Unmarshal(responseBody, &telegramResp); err != nil {
		log.Printf("解析 Telegram 响应失败: %v", err)
	}

	var messageID int64
	if params.Action == TelegramActionSend && telegramResp.OK {
		messageID = telegramResp.Result.MessageID
	}

	target := "Telegram"
	if config.Name != "" {
		target = fmt.Sprintf("Telegram[%s]", config.Name)
	}

	actionDesc := map[TelegramAction]string{
		TelegramActionSend:   "消息发送成功",
		TelegramActionEdit:   "消息编辑成功",
		TelegramActionDelete: "消息删除成功",
	}
	log.Printf("%s %s (message_id: %d)", target, actionDesc[params.Action], messageID)
	return messageID, nil
}

func buildSendMessagePayload(config *TelegramConfig, params *TelegramMessageParams) ([]byte, error) {
	type linkPreviewOptions struct {
		IsDisabled bool `json:"is_disabled"`
	}

	type telegramMessage struct {
		ChatID             string              `json:"chat_id"`
		Text               string              `json:"text"`
		ParseMode          string              `json:"parse_mode,omitempty"`
		LinkPreviewOptions *linkPreviewOptions `json:"link_preview_options,omitempty"`
	}

	msg := telegramMessage{
		ChatID: config.ChatID,
		Text:   params.Text,
	}

	if params.ParseMode != "" {
		switch strings.ToLower(params.ParseMode) {
		case "text/html":
			msg.ParseMode = "HTML"
		case "text/markdown":
			msg.ParseMode = "Markdown"
		case "text/x-markdown-v2":
			msg.ParseMode = "MarkdownV2"
		default:
			upperMode := strings.ToUpper(params.ParseMode)
			if upperMode == "MARKDOWNV2" {
				msg.ParseMode = "MarkdownV2"
			} else if upperMode == "MARKDOWN" {
				msg.ParseMode = "Markdown"
			} else {
				msg.ParseMode = upperMode
			}
		}
	}

	if msg.ParseMode == "" && config.ParseMode != "" {
		upperMode := strings.ToUpper(config.ParseMode)
		if upperMode == "MARKDOWNV2" {
			msg.ParseMode = "MarkdownV2"
		} else if upperMode == "MARKDOWN" {
			msg.ParseMode = "Markdown"
		} else {
			msg.ParseMode = upperMode
		}
	}

	if !params.EnablePreview && !config.EnablePreview {
		msg.LinkPreviewOptions = &linkPreviewOptions{
			IsDisabled: true,
		}
	}

	return json.Marshal(msg)
}

func buildEditMessagePayload(params *TelegramMessageParams) ([]byte, error) {
	type editMessage struct {
		ChatID    string `json:"chat_id"`
		MessageID int64  `json:"message_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode,omitempty"`
	}

	msg := editMessage{
		ChatID:    params.ChatID,
		MessageID: params.MessageID,
		Text:      params.Text,
	}

	if params.ParseMode != "" {
		upperMode := strings.ToUpper(params.ParseMode)
		if upperMode == "MARKDOWNV2" {
			msg.ParseMode = "MarkdownV2"
		} else if upperMode == "MARKDOWN" {
			msg.ParseMode = "Markdown"
		} else {
			msg.ParseMode = upperMode
		}
	}

	return json.Marshal(msg)
}

func buildDeleteMessagePayload(params *TelegramMessageParams) ([]byte, error) {
	type deleteMessage struct {
		ChatID    string `json:"chat_id"`
		MessageID int64  `json:"message_id"`
	}

	msg := deleteMessage{
		ChatID:    params.ChatID,
		MessageID: params.MessageID,
	}

	return json.Marshal(msg)
}

func sendToWebhook(config *WebhookConfig, message string) error {
	req, err := http.NewRequest("POST", config.URL, bytes.NewBufferString(message))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}

	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/plain")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送到 Webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Webhook 返回错误状态 %s: %s", resp.Status, string(responseBody))
	}

	log.Printf("Webhook 消息发送成功: %s", config.URL)
	return nil
}

func logMessage(config *LogConfig, message string) {
	severityPrefix := map[Severity]string{
		SeverityNormal:    "[一般]",
		SeverityImportant: "[重要]",
		SeverityUrgent:    "[紧急]",
	}

	prefix := severityPrefix[config.Severity]
	if prefix == "" {
		prefix = "[消息]"
	}

	log.Printf("%s %s", prefix, message)
}
