// 配置可视化工具
package main

import (
	"fmt"
	"sort"
	"strings"
)

// ViewConfig 可视化配置
func ViewConfig(config *Config) {
	fmt.Println("    Webhook 路由配置视图    ")
	fmt.Println()

	fmt.Printf("监听地址: %s\n", config.Listen)
	fmt.Println()

	fmt.Println("路由端点:")
	for _, route := range config.Routes {
		fmt.Printf("  %s -> %s\n", route.Path, route.Node)
	}
	fmt.Println()

	fmt.Println("节点拓扑图:")
	fmt.Println()

	visitedGlobal := make(map[string]bool)
	for _, route := range config.Routes {
		if !visitedGlobal[route.Node] {
			printNodeTree(config, route.Node, "", true, visitedGlobal, make(map[string]bool))
		}
	}

	orphanNodes := findOrphanNodes(config)
	if len(orphanNodes) > 0 {
		fmt.Println()
		fmt.Println("未使用的节点:")
		for _, nodeName := range orphanNodes {
			node := config.Nodes[nodeName]
			nodeType := getNodeTypeDesc(node)
			fmt.Printf("  • %s (%s)\n", nodeName, nodeType)
		}
	}
}

func printNodeTree(config *Config, nodeName string, prefix string, isLast bool, visitedGlobal map[string]bool, visitedPath map[string]bool) {
	node, exists := config.Nodes[nodeName]
	if !exists {
		return
	}

	connector := "├─"
	if isLast {
		connector = "└─"
	}

	nodeDesc := getNodeDescription(nodeName, node)

	if visitedPath[nodeName] {
		fmt.Printf("%s%s %s (循环引用)\n", prefix, connector, nodeDesc)
		return
	}

	fmt.Printf("%s%s %s\n", prefix, connector, nodeDesc)

	visitedGlobal[nodeName] = true
	visitedPath[nodeName] = true

	nextNodes := getNextNodes(node)
	if len(nextNodes) > 0 {
		newPrefix := prefix
		if isLast {
			newPrefix += "  "
		} else {
			newPrefix += "│ "
		}

		for i, nextNode := range nextNodes {
			isLastChild := i == len(nextNodes)-1
			newVisitedPath := make(map[string]bool)
			for k, v := range visitedPath {
				newVisitedPath[k] = v
			}
			printNodeTree(config, nextNode, newPrefix, isLastChild, visitedGlobal, newVisitedPath)
		}
	}
}

func getNextNodes(node Node) []string {
	if node.Log != nil && len(node.Log.Next) > 0 {
		return node.Log.Next
	}
	return node.Next
}

func getNodeDescription(name string, node Node) string {
	nodeType := getNodeTypeDesc(node)
	return fmt.Sprintf("%s [%s]", name, nodeType)
}

func getNodeTypeDesc(node Node) string {
	if node.Wecom != nil {
		if node.Wecom.Name != "" {
			return fmt.Sprintf("企业微信: %s", node.Wecom.Name)
		}
		return "企业微信"
	}
	if node.Telegram != nil {
		if node.Telegram.Name != "" {
			return fmt.Sprintf("Telegram: %s", node.Telegram.Name)
		}
		return "Telegram"
	}
	if node.Webhook != nil {
		return fmt.Sprintf("Webhook: %s", node.Webhook.URL)
	}
	if node.Log != nil {
		severityName := map[Severity]string{
			SeverityNormal:    "一般",
			SeverityImportant: "重要",
			SeverityUrgent:    "紧急",
		}
		name := severityName[node.Log.Severity]
		if name == "" {
			name = string(node.Log.Severity)
		}
		return fmt.Sprintf("日志: %s", name)
	}
	if len(node.Next) > 0 {
		return "虚拟节点"
	}
	return "叶子节点"
}

func findOrphanNodes(config *Config) []string {
	usedNodes := make(map[string]bool)

	for _, route := range config.Routes {
		markUsedNodes(config, route.Node, usedNodes, make(map[string]bool))
	}

	var orphans []string
	for nodeName := range config.Nodes {
		if !usedNodes[nodeName] {
			orphans = append(orphans, nodeName)
		}
	}

	sort.Strings(orphans)
	return orphans
}

func markUsedNodes(config *Config, nodeName string, used map[string]bool, visited map[string]bool) {
	if visited[nodeName] {
		return
	}

	visited[nodeName] = true
	used[nodeName] = true

	node, exists := config.Nodes[nodeName]
	if !exists {
		return
	}

	nextNodes := getNextNodes(node)
	for _, next := range nextNodes {
		markUsedNodes(config, next, used, visited)
	}
}

// ViewConfigSimple 简单视图,只显示路由映射
func ViewConfigSimple(config *Config) {
	maxPathLen := 0
	for _, route := range config.Routes {
		if len(route.Path) > maxPathLen {
			maxPathLen = len(route.Path)
		}
	}

	fmt.Println("路由映射:")
	for _, route := range config.Routes {
		padding := strings.Repeat(" ", maxPathLen-len(route.Path))
		node := config.Nodes[route.Node]
		nodeDesc := getNodeTypeDesc(node)
		fmt.Printf("  %s%s  →  %s [%s]\n", route.Path, padding, route.Node, nodeDesc)
	}
}
