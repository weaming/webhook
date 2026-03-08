// Webhook 服务主程序，支持多种通知目标和扇出路由
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

func handleWebhook(router *MessageRouter, nodeName string, nodeConfig *Node) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestBody, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
			return
		}

		messageContent := string(requestBody)
		contentType := c.ContentType()

		messageIDStr := c.Query("message_id")
		var messageID int64
		if messageIDStr != "" {
			messageID, err = strconv.ParseInt(messageIDStr, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 message_id 参数"})
				return
			}
		}

		var action TelegramAction
		switch c.Request.Method {
		case "POST":
			action = TelegramActionSend
			log.Printf("收到消息 [节点: %s, 方法: POST, 类型: %s]: %s", nodeName, contentType, messageContent)
		case "PUT":
			action = TelegramActionEdit
			log.Printf("收到消息 [节点: %s, 方法: PUT, 类型: %s]: %s", nodeName, contentType, messageContent)
		case "DELETE":
			action = TelegramActionDelete
			log.Printf("收到删除请求 [节点: %s, 方法: DELETE]", nodeName)
		default:
			c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "不支持的 HTTP 方法"})
			return
		}

		if err := router.SendWithAction(nodeName, messageContent, contentType, messageID, action); err != nil {
			log.Printf("操作失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "success"})
	}
}

func runServer(configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	router := NewMessageRouter(config)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	for _, route := range config.Routes {
		nodeConfig, exists := config.Nodes[route.Node]
		var nodePtr *Node
		if exists {
			nodePtr = &nodeConfig
		}

		r.POST(route.Path, handleWebhook(router, route.Node, nodePtr))
		r.PUT(route.Path, handleWebhook(router, route.Node, nodePtr))
		r.DELETE(route.Path, handleWebhook(router, route.Node, nodePtr))
		log.Printf("注册路由: %s -> %s (POST/PUT/DELETE)", route.Path, route.Node)
	}

	log.Printf("启动 Webhook 服务器: %s", config.Listen)
	return r.Run(config.Listen)
}

func generateConfig(outputPath string) error {
	configContent := GenerateDefaultConfig()

	if err := os.WriteFile(outputPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	log.Printf("配置文件已生成: %s", outputPath)
	return nil
}

func viewConfig(configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	ViewConfig(config)
	return nil
}

func printUsage() {
	fmt.Println("用法:")
	fmt.Println("  webhook [选项]")
	fmt.Println("  webhook <子命令> [参数]")
	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  -c, --config <路径>  配置文件路径 (默认: webhook.yaml)")
	fmt.Println()
	fmt.Println("子命令:")
	fmt.Println("  gen [配置文件路径]   生成配置文件模板 (默认: webhook.yaml)")
	fmt.Println("  view [配置文件路径]  查看配置的路由拓扑图 (默认: webhook.yaml)")
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		configPath := os.Getenv("CONFIG_PATH")
		if configPath == "" {
			configPath = "webhook.yaml"
		}

		if err := runServer(configPath); err != nil {
			log.Fatalf("服务器运行失败: %v", err)
		}
		return
	}

	switch args[0] {
	case "gen":
		outputPath := "webhook.yaml"
		if len(args) > 1 {
			outputPath = args[1]
		}

		if err := generateConfig(outputPath); err != nil {
			log.Fatalf("生成配置文件失败: %v", err)
		}

	case "view":
		configPath := "webhook.yaml"
		if len(args) > 1 {
			configPath = args[1]
		}

		if err := viewConfig(configPath); err != nil {
			log.Fatalf("查看配置失败: %v", err)
		}

	case "-c", "--config":
		if len(args) < 2 {
			printUsage()
			os.Exit(1)
		}

		if err := runServer(args[1]); err != nil {
			log.Fatalf("服务器运行失败: %v", err)
		}

	case "-h", "--help", "help":
		printUsage()

	default:
		fmt.Printf("未知命令: %s\n\n", args[0])
		printUsage()
		os.Exit(1)
	}
}
