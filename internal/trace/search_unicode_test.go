package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchUnicode_Chinese(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			Function:  "函数名称", // Chinese characters
			Error:     "错误信息",
			EventData: "事件数据",
		},
	}

	// Test searching Chinese characters
	engine.SetQuery("函数")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "1", matches[0].NodeID)

	engine.SetQuery("错误")
	matches = engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchUnicode_Cyrillic(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:    "1",
			Error: "Ошибка выполнения", // Cyrillic
		},
	}

	engine.SetQuery("Ошибка")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchUnicode_French(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "événement créé", // French with accents
		},
	}

	engine.SetQuery("événement")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchUnicode_Arabic(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:    "1",
			Error: "خطأ في التنفيذ", // Arabic
		},
	}

	engine.SetQuery("خطأ")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchUnicode_Emoji(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "Transfer complete ✅",
		},
		{
			ID:    "2",
			Error: "Failed ❌",
		},
	}

	engine.SetQuery("✅")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "1", matches[0].NodeID)

	engine.SetQuery("❌")
	matches = engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "2", matches[0].NodeID)
}

func TestSearchUnicode_Mixed(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			Function:  "transfer_资金",
			EventData: "Événement créé ✅",
		},
	}

	// Search for Chinese part
	engine.SetQuery("资金")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))

	// Search for French part
	engine.SetQuery("Événement")
	matches = engine.Search(nodes)
	assert.Equal(t, 1, len(matches))

	// Search for emoji
	engine.SetQuery("✅")
	matches = engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Dollar(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:         "1",
			ContractID: "CA$H_ADDR#123",
			Function:   "transfer*",
		},
	}

	// Should treat special chars literally
	engine.SetQuery("$H")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Asterisk(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:       "1",
			Function: "transfer*",
		},
	}

	engine.SetQuery("*")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Hash(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:         "1",
			ContractID: "ADDR#123",
		},
	}

	engine.SetQuery("#123")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Brackets(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "array[0]",
		},
		{
			ID:        "2",
			EventData: "map{key: value}",
		},
	}

	engine.SetQuery("[0]")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "1", matches[0].NodeID)

	engine.SetQuery("{key")
	matches = engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
	assert.Equal(t, "2", matches[0].NodeID)
}

func TestSearchSpecialChars_Dots(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:       "1",
			Function: "contract.transfer",
		},
	}

	// Dot should be treated literally, not as regex wildcard
	engine.SetQuery("contract.transfer")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Parentheses(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:       "1",
			Function: "transfer(amount)",
		},
	}

	engine.SetQuery("(amount)")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Pipe(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "value1|value2",
		},
	}

	engine.SetQuery("|")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Plus(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "balance+100",
		},
	}

	engine.SetQuery("+100")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Question(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:    "1",
			Error: "Invalid input?",
		},
	}

	engine.SetQuery("?")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Backslash(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "path\\to\\file",
		},
	}

	engine.SetQuery("\\to\\")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_Caret(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "value^2",
		},
	}

	engine.SetQuery("^2")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchUnicode_CaseInsensitive(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "Événement CRÉÉ",
		},
	}

	// Case insensitive should work with unicode
	engine.SetQuery("événement")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))

	engine.SetQuery("créé")
	matches = engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchUnicode_LongString(t *testing.T) {
	engine := NewSearchEngine()

	nodes := []*TraceNode{
		{
			ID:        "1",
			EventData: "这是一个很长的中文字符串，包含了很多汉字和标点符号。",
		},
	}

	engine.SetQuery("很长的中文")
	matches := engine.Search(nodes)
	assert.Equal(t, 1, len(matches))
}

func TestSearchSpecialChars_AllRegexChars(t *testing.T) {
	engine := NewSearchEngine()

	// Test all regex special characters are treated literally
	specialChars := []string{
		".", "*", "+", "?", "^", "$", "(", ")", "[", "]", "{", "}", "|", "\\",
	}

	for _, char := range specialChars {
		nodes := []*TraceNode{
			{
				ID:        "1",
				EventData: "test" + char + "value",
			},
		}

		engine.SetQuery(char)
		matches := engine.Search(nodes)
		assert.Equal(t, 1, len(matches), "Failed to find special char: %s", char)
	}
}
