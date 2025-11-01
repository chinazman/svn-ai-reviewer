package ai

// ReviewJSON AI 返回的 JSON 结构
type ReviewJSON struct {
	Summary string  `json:"summary"`
	Score   int     `json:"score"`
	Issues  []Issue `json:"issues"`
}

// Issue 代码问题
type Issue struct {
	Severity    string `json:"severity"`    // high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}
