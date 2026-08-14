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
		fmt.Printf("%s: ", cfg.BotName)
		thinkStyle, resetStyle := false, false
		_, err = b.Chat(context.Background(), input,
			func(text string) {
				if !thinkStyle {
					fmt.Print("\x1b[3;90m🧠 ")
					thinkStyle, resetStyle = true, true
				}
				fmt.Print(text)
			},
			func(text string) {
				if resetStyle {
					fmt.Print("\x1b[0m")
					thinkStyle, resetStyle = false, false
				}
				fmt.Print(text)
			},
		)
		fmt.Println()
		if err != nil {
			fmt.Printf("⚠️  %v\n", err)
			continue
		}
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
		fmt.Println("  /think <on|off>    开启或关闭当前供应商的思考模式")
		fmt.Println("  /effort <low|high|max>  设置思考强度")
		fmt.Println("  /info              显示当前供应商、模型和思考配置")
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
	case "/think":
		if len(args) == 0 {
			fmt.Println("用法: /think <on|off>")
			return true
		}
		v := map[string]string{"on": "enabled", "off": "disabled"}[args[0]]
		if err := b.SetThinking(v); err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return true
		}
		fmt.Printf("思考模式已%s\n", map[string]string{"enabled": "开启", "disabled": "关闭"}[v])
	case "/effort":
		if len(args) == 0 {
			fmt.Println("用法: /effort <low|high|max>")
			return true
		}
		if err := b.SetEffort(args[0]); err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return true
		}
		fmt.Printf("思考强度已设置为 %s\n", args[0])
	case "/info":
		provider, model := b.Current()
		thinking, effort := b.ThinkingConfig()
		if thinking == "" {
			thinking = "enabled(默认)"
		}
		if effort == "" {
			effort = "high(默认)"
		}
		fmt.Printf("供应商: %s, 模型: %s, 思考模式: %s, 思考强度: %s\n", provider, model, thinking, effort)
	default:
		fmt.Printf("未知命令: %s，输入 /help 查看命令列表\n", cmd)
	}
	return true
}
