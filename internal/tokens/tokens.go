package tokens

import "github.com/pkoukk/tiktoken-go"

var tke *tiktoken.Tiktoken

func getEncoding() *tiktoken.Tiktoken {
	if tke == nil {
		t, err := tiktoken.GetEncoding("o200k_base")
		if err != nil {
			return nil
		}
		tke = t
	}
	return tke
}

// Tokenize 将文本编码为 o200k_base token id 列表（去重）。
// 编码器初始化失败时返回 nil。
func Tokenize(text string) []int64 {
	if text == "" {
		return nil
	}
	enc := getEncoding()
	if enc == nil {
		return nil
	}
	seen := make(map[int64]bool)
	var out []int64
	for _, id := range enc.Encode(text, nil, nil) {
		tid := int64(id)
		if seen[tid] {
			continue
		}
		seen[tid] = true
		out = append(out, tid)
	}
	return out
}

// Count 统计文本的 token 数量；编码器初始化失败时回退为字符数/2 估算。
func Count(text string) int64 {
	if text == "" {
		return 0
	}
	enc := getEncoding()
	if enc == nil {
		return int64(len([]rune(text)) / 2)
	}
	return int64(len(enc.Encode(text, nil, nil)))
}
