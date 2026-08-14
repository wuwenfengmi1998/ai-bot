package cli

import (
	"reflect"
	"testing"
)

func TestComplete(t *testing.T) {
	models := []string{"deepseek-v4-flash", "deepseek-v4-pro", "gpt-4o"}
	cases := []struct {
		name string
		line string
		want []string
	}{
		{"空行返回全部命令", "", commands},
		{"命令前缀", "/us", []string{"/use"}},
		{"模型补全", "/use deepseek", []string{"deepseek-v4-flash", "deepseek-v4-pro"}},
		{"模型无匹配", "/use claude", nil},
		{"think 补全", "/think o", []string{"on", "off"}},
		{"effort 补全", "/effort h", []string{"high"}},
		{"未知命令不补全", "/foo a", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Complete(c.line, models)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Complete(%q) = %v, want %v", c.line, got, c.want)
			}
		})
	}
}
