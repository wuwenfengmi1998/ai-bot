package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"myaibot/internal/bot"
	"myaibot/internal/store"
)

type Handler struct {
	bot *bot.Bot
	db  *sql.DB
}

func New(b *bot.Bot, db *sql.DB) *Handler {
	return &Handler{bot: b, db: db}
}

func formatWindow(n int64) string {
	switch {
	case n <= 0:
		return "未配置"
	case n >= 1048576:
		return fmt.Sprintf("%dM tokens", n/1048576)
	case n >= 1024:
		return fmt.Sprintf("%dK tokens", n/1024)
	default:
		return fmt.Sprintf("%d tokens", n)
	}
}

func thousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func (h *Handler) Handle(input string) bool {
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
		fmt.Println("  /context           打印当前聊天上下文")
		fmt.Println("  /tools             列出可用工具")
		fmt.Println("  /sessions          列出历史会话")
		fmt.Println("  /session <id>      切换到历史会话，如 /session 3")
		fmt.Println("  /info              显示当前供应商、模型和思考配置")
		fmt.Println("  /exit              退出")
	case "/models":
		for _, m := range h.bot.Models() {
			fmt.Println("  " + m)
		}
	case "/use":
		if len(args) == 0 {
			fmt.Println("用法: /use <模型>，如 /use deepseek-chat")
			return true
		}
		if err := h.bot.SwitchModel(args[0]); err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return true
		}
		provider, model := h.bot.Current()
		fmt.Printf("已切换到 %s/%s (对话历史已保留)\n", provider, model)
	case "/think":
		if len(args) == 0 {
			fmt.Println("用法: /think <on|off>")
			return true
		}
		v := map[string]string{"on": "enabled", "off": "disabled"}[args[0]]
		if err := h.bot.SetThinking(v); err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return true
		}
		fmt.Printf("思考模式已%s\n", map[string]string{"enabled": "开启", "disabled": "关闭"}[v])
	case "/effort":
		if len(args) == 0 {
			fmt.Println("用法: /effort <low|high|max>")
			return true
		}
		if err := h.bot.SetEffort(args[0]); err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return true
		}
		fmt.Printf("思考强度已设置为 %s\n", args[0])
	case "/context":
		fmt.Print(h.bot.ContextDump())
		used, total := h.bot.ContextStats()
		if total <= 0 {
			fmt.Printf("上下文: 约 %s tokens（窗口大小未配置）\n", thousands(used))
			return true
		}
		pct := float64(used) / float64(total) * 100
		fmt.Printf("上下文窗口使用: %s / %s tokens (%.2f%%)\n", thousands(used), thousands(total), pct)
	case "/tools":
		for _, t := range h.bot.Tools() {
			fmt.Println("  " + t)
		}
	case "/sessions":
		list, err := store.ListSessions(h.db)
		if err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return true
		}
		if len(list) == 0 {
			fmt.Println("暂无历史会话")
			return true
		}
		for _, s := range list {
			fmt.Printf("  #%d  %s  (%d 条消息)\n", s.ID, s.CreatedAt.Format("2006-01-02 15:04:05"), s.MessageCount)
		}
	case "/session":
		if len(args) == 0 {
			fmt.Println("用法: /session <id>，如 /session 3")
			return true
		}
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			fmt.Printf("无效的会话 id: %s\n", args[0])
			return true
		}
		sess, err := store.LoadSession(h.db, id)
		if err != nil {
			fmt.Printf("⚠️  %v\n", err)
			return true
		}
		if sess == nil {
			fmt.Printf("会话 #%d 不存在\n", id)
			return true
		}
		h.bot.RestoreSession(sess)
		fmt.Printf("已切换到会话 #%d (%d 条消息)\n", id, len(sess.Messages))
	case "/info":
		provider, model := h.bot.Current()
		thinking, effort := h.bot.ThinkingConfig()
		tool, vision := h.bot.CurrentRoles()
		if thinking == "" {
			thinking = "enabled(默认)"
		}
		if effort == "" {
			effort = "high(默认)"
		}
		fmt.Printf("供应商: %s, 模型: %s, 思考模式: %s, 思考强度: %s\n", provider, model, thinking, effort)
		fmt.Printf("上下文窗口: %s\n", formatWindow(h.bot.ContextWindow()))
		fmt.Printf("工具调用AI: %s\n图片识别AI: %s\n", tool, vision)
	default:
		fmt.Printf("未知命令: %s，输入 /help 查看命令列表\n", cmd)
	}
	return true
}
