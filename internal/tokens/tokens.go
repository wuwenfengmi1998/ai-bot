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

// Token 表示一个 o200k_base token：ID 与对应的符号文本。
type Token struct {
	ID   int64
	Text string
}

// Tokenize 将文本编码为 token 列表（去重），Text 为 token 对应的符号。
// 编码器初始化失败时返回 nil。
func Tokenize(text string) []Token {
	if text == "" {
		return nil
	}
	enc := getEncoding()
	if enc == nil {
		return nil
	}
	seen := make(map[int64]bool)
	var out []Token
	for _, id := range enc.Encode(text, nil, nil) {
		tid := int64(id)
		if seen[tid] {
			continue
		}
		seen[tid] = true
		out = append(out, Token{ID: tid, Text: enc.Decode([]int{int(tid)})})
	}
	return out
}

// TokenIDs 提取 Token 列表的 id。
func TokenIDs(ts []Token) []int64 {
	if len(ts) == 0 {
		return nil
	}
	out := make([]int64, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

// Text 返回 token id 对应的符号文本；未知 id 返回空串。
func Text(id int64) string {
	enc := getEncoding()
	if enc == nil {
		return ""
	}
	return enc.Decode([]int{int(id)})
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
