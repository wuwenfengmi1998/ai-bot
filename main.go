package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterh/liner"

	"myaibot/internal/bot"
	"myaibot/internal/cli"
	"myaibot/internal/config"
	"myaibot/internal/store"
)

func main() {
	os.Setenv("TIKTOKEN_CACHE_DIR", filepath.Join("data", "tiktoken"))
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	autoFetchModels(cfg)

	db, err := store.Open(&cfg.Database)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	fmt.Printf("💾 数据库已连接 (%s)\n", cfg.Database.Driver)

	b, err := bot.New(cfg)
	if err != nil {
		fmt.Printf("⚠️  %v\n", err)
		fmt.Println("请填写工具配置文件后重新启动。")
		store.Close(db)
		os.Exit(1)
	}

	if err := store.Migrate(db, cfg.Database.Driver); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	if sess, err := store.LoadLatestSession(db); err != nil {
		fmt.Printf("⚠️ 读取上次会话失败: %v\n", err)
	} else if sess != nil && len(sess.Messages) > 0 {
		b.RestoreSession(sess)
		fmt.Printf("💬 已恢复上次会话 (%d 条消息)\n", len(sess.Messages))
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

	h := cli.New(b, db)
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

	saveSession(db, b)
	store.Close(db)
}

func autoFetchModels(cfg *config.Config) {
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if !p.AutoFetchModels {
			continue
		}
		if p.APIKey == "" {
			fmt.Printf("⚠️  供应商 %s 启用了 auto_fetch_models 但未配置 api_key，跳过自动获取\n", p.Name)
			continue
		}
		before := append([]config.ModelConfig(nil), p.Models...)
		if err := config.FetchModels(context.Background(), p); err != nil {
			fmt.Printf("⚠️  自动获取模型失败 (供应商 %s): %v，使用现有模型列表\n", p.Name, err)
			continue
		}
		fmt.Printf("📚 已从 API 获取模型列表 (供应商 %s): %d 个模型\n", p.Name, len(p.Models))
		if !config.ModelsEqual(before, p.Models) {
			if err := config.Save(cfg); err != nil {
				fmt.Printf("⚠️  模型列表写回配置失败: %v\n", err)
			}
		}
	}
}

func saveSession(db *sql.DB, b *bot.Bot) {
	msgs := b.SessionMessages()
	if len(msgs) == 0 {
		return
	}
	provider, model := b.Current()
	sess := &store.Session{
		Provider:     provider,
		Model:        model,
		SystemPrompt: "",
		Messages:     msgs,
	}
	if _, err := store.SaveSession(db, sess); err != nil {
		fmt.Printf("⚠️  保存会话失败: %v\n", err)
		return
	}
	fmt.Printf("💾 会话已保存 (%d 条消息)\n", len(msgs))
}
