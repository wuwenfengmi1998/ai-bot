package bot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go"

	"myaibot/internal/config"
	"myaibot/internal/store"
)

func TestExtractMemories(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("请求路径 = %q, want /chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"choices":[{"delta":{"reasoning_content":"用户提到了喝咖啡和Go开发，值得长期记住。","content":"{\"memories\":[{\"content\":\"用户喜欢喝咖啡\",\"category\":\"preference\",\"importance\":7},{\"content\":\"\",\"category\":\"fact\",\"importance\":99},{\"content\":\"用户是Go开发者\",\"category\":\"fact\",\"importance\":-3}]}"}}]}` + "\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	b := &Bot{
		clients:        make(map[string]*openai.Client),
		memoryProvider: &config.Provider{Name: "mem", APIKey: "sk-test", BaseURL: srv.URL},
		memoryModel:    "m",
		history: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("我喜欢喝咖啡，是个 Go 开发者"),
		},
	}
	var reasoning strings.Builder
	ms, err := b.ExtractMemories(context.Background(), nil, func(text string) {
		reasoning.WriteString(text)
	})
	if err != nil {
		t.Fatalf("ExtractMemories 出错: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("提取数量 = %d, want 2（空内容跳过）", len(ms))
	}
	if ms[0].Content != "用户喜欢喝咖啡" || ms[0].Category != "preference" || ms[0].Importance != 7 {
		t.Errorf("记忆0异常: %+v", ms[0])
	}
	if ms[1].Content != "用户是Go开发者" || ms[1].Importance != 0 {
		t.Errorf("重要度应钳制到 0-10: %+v", ms[1])
	}
	if reasoning.Len() == 0 {
		t.Error("思考流应被回调")
	}
}

func TestExtractMemoriesNoHistory(t *testing.T) {
	b := &Bot{
		clients:        make(map[string]*openai.Client),
		memoryProvider: &config.Provider{Name: "mem", APIKey: "sk-test"},
		memoryModel:    "m",
	}
	if _, err := b.ExtractMemories(context.Background(), nil, nil); err == nil || !strings.Contains(err.Error(), "没有可提取") {
		t.Errorf("无历史应报错: %v", err)
	}
}

func TestExtractMemoriesNoAPIKey(t *testing.T) {
	b := &Bot{
		clients:        make(map[string]*openai.Client),
		memoryProvider: &config.Provider{Name: "mem"},
		memoryModel:    "m",
		history: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hi"),
		},
	}
	if _, err := b.ExtractMemories(context.Background(), nil, nil); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Errorf("无 api_key 应报错: %v", err)
	}
}

func TestExtractMemoriesEmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"memories\\\":[]}\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	b := &Bot{
		clients:        make(map[string]*openai.Client),
		memoryProvider: &config.Provider{Name: "mem", APIKey: "sk-test", BaseURL: srv.URL},
		memoryModel:    "m",
		history: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hello"),
		},
	}
	ms, err := b.ExtractMemories(context.Background(), []store.Memory{{Content: "已有记忆"}}, nil)
	if err != nil {
		t.Fatalf("ExtractMemories 出错: %v", err)
	}
	if len(ms) != 0 {
		t.Errorf("空结果应返回空切片, got %+v", ms)
	}
}
