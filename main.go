package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/peterh/liner"

	"myaibot/internal/bot"
	"myaibot/internal/cli"
	"myaibot/internal/config"
	"myaibot/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := store.Open(&cfg.Database)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer store.Close(db)
	fmt.Printf("💾 数据库已连接 (%s)\n", cfg.Database.Driver)

	b, err := bot.New(cfg)
	if err != nil {
		fmt.Printf("⚠️  %v\n", err)
		fmt.Println("请填写工具配置文件后重新启动。")
		os.Exit(1)
	}
	provider, model := b.Current()
	fmt.Printf("🤖 %s 已启动 (供应商: %s, 模型: %s)。输入问题开始对话，输入 /help 查看命令。\n",
		cfg.BotName, provider, model)

	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetCompleter(func(s string) []string {
		return cli.Complete(s, b.Models())
	})

	h := cli.New(b)
	for {
		input, err := line.Prompt("你: ")
		if errors.Is(err, io.EOF) || errors.Is(err, liner.ErrPromptAborted) {
			fmt.Println("再见！")
			break
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		line.AppendHistory(input)
		if strings.HasPrefix(input, "/") {
			if !h.Handle(input) {
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
			func(name, args string) {
				if resetStyle {
					fmt.Print("\x1b[0m")
					thinkStyle, resetStyle = false, false
				}
				fmt.Printf("🔧 调用工具: %s %s\n", name, args)
			},
			func(text string) {
				if !thinkStyle {
					fmt.Print("\x1b[3;90m🔧 ")
					thinkStyle, resetStyle = true, true
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
}
