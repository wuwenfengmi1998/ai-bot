package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"myaibot/internal/config"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(args json.RawMessage) (string, error)
}

// Configurable 是可配置工具的接口：配置文件 data/tools/<Name()>.yaml
// 缺失时生成默认模板并提醒，存在时注入配置。
type Configurable interface {
	Tool
	DefaultConfig() map[string]any
	Configure(cfg map[string]any) error
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(t ...Tool) *Registry {
	r := &Registry{tools: make(map[string]Tool)}
	for _, tool := range t {
		r.tools[tool.Name()] = tool
	}
	return r
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, fmt.Sprintf("%s - %s", name, r.tools[name].Description()))
	}
	return out
}

func (r *Registry) ParamList() []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name(),
				Description: param.NewOpt(t.Description()),
				Parameters:  shared.FunctionParameters(t.Parameters()),
			},
		})
	}
	return out
}

func (r *Registry) Execute(name string, args json.RawMessage) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("未知工具: %s", name)
	}
	return t.Execute(args)
}

// InitConfigs 为可配置工具注入配置。
// 配置文件缺失时生成默认模板并返回提醒；配置非法时返回错误。
func (r *Registry) InitConfigs() error {
	var missing []string
	for name, t := range r.tools {
		c, ok := t.(Configurable)
		if !ok {
			continue
		}
		cfg, found, err := config.LoadToolConfig(name)
		if err != nil {
			return fmt.Errorf("工具 %s 配置加载失败: %w", name, err)
		}
		if !found {
			if err := config.WriteDefaultToolConfig(name, c.DefaultConfig()); err != nil {
				return fmt.Errorf("工具 %s 默认配置生成失败: %w", name, err)
			}
			missing = append(missing, fmt.Sprintf("工具 %s 缺少配置，已生成模板 %s，请填写后重启", name, config.ToolConfigPath(name)))
			continue
		}
		if err := c.Configure(cfg); err != nil {
			return fmt.Errorf("工具 %s 配置无效: %w", name, err)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s", strings.Join(missing, "\n"))
	}
	return nil
}
