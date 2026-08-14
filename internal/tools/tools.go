package tools

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(args json.RawMessage) (string, error)
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
