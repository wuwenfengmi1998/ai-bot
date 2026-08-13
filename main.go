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
	if cfg.APIKey == "" {
		log.Println("提示: 未配置 api_key，请编辑 data/config.yaml")
	}
	fmt.Printf("🤖 %s 已启动 (模型: %s)。输入问题开始对话，输入 /exit 退出。\n", cfg.BotName, cfg.Model)

	b := bot.New(cfg)
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
		if input == "/exit" || input == "/quit" {
			fmt.Println("再见！")
			break
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
