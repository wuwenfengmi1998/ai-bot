package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"myaibot/internal/bot"
	"myaibot/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	b := bot.New(cfg)
	provider, model := b.Current()
	fmt.Printf("🤖 %s 已启动 (供应商: %s, 模型: %s)。输入问题开始对话，输入 /help 查看命令。\n",
		cfg.BotName, provider, model)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("你: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "/") {
			if !handleCommand(b, input) {
				break
			}
			continue
		}
		answer, err := b.Chat(context.Background(), input)
		if err != nil {
			fmt.Printf("⚠️  %v\n", err)
			continue
		}
		fmt.Printf("%s: %s\n", cfg.BotName, answer)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func handleCommand(b *bot.Bot, input string) bool {
	fields := strings.Fields(input)
	cmd, args := fields[0], fields[1:]
	switch cmd {
	case "/exit", "/quit":
		fmt.Println("再见！")
		return false
	case "/help":
		fmt.Println("命令列表:")
		fmt.Println("  /models            列出所有供应商和模型")
		fmt.Println("  /use <模型>        切换模型，如 /use deepseek-chat 或 /use deepseek/deepseek-chat")
		fmt.Println("  /info              显示当前供应商和模型")
		fmt.Println("  /exit              退出")
	case "/models":
		for _, m := range b.Models() {
			fmt.Println("  " + m)
		}
	case "/use":
		if len(args) == 0 {
			fmt.Println("用法: /use <模型>，如 /use deepseek-chat")
			return true
		}
		if err := b.SwitchModel(args[0]); err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return true
		}
		provider, model := b.Current()
		fmt.Printf("已切换到 %s/%s (对话历史已保留)\n", provider, model)
	case "/info":
		provider, model := b.Current()
		fmt.Printf("供应商: %s, 模型: %s\n", provider, model)
	default:
		fmt.Printf("未知命令: %s，输入 /help 查看命令列表\n", cmd)
	}
	return true
}
