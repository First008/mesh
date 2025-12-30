package vectorstore

import (
	"testing"
)

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "simple query",
			query:    "How does the authentication service work?",
			expected: []string{"does", "authentication", "service", "work"},
		},
		{
			name:     "with Go common tokens filtered",
			query:    "ctx err func return string int",
			expected: []string{}, // All filtered by stoplist
		},
		{
			name:     "mixed keywords",
			query:    "SignTransaction method in blockchain service",
			expected: []string{"signtransaction", "method", "blockchain"},
		},
		{
			name:     "camelCase and snake_case",
			query:    "getUserData get_user_id",
			expected: []string{"getuserdata", "get_user_id"},
		},
		{
			name:     "short words filtered",
			query:    "a an in on to API",
			expected: []string{}, // Short words and stopwords filtered
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractKeywords(tt.query)
			if len(result) != len(tt.expected) {
				t.Errorf("extractKeywords() returned %d keywords, expected %d: got %v, want %v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}
			for i, keyword := range result {
				if keyword != tt.expected[i] {
					t.Errorf("extractKeywords()[%d] = %v, want %v", i, keyword, tt.expected[i])
				}
			}
		})
	}
}

func TestCalculateKeywordScore(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		keywords   []string
		fileLength int
		minScore   float32
		maxScore   float32
	}{
		{
			name:       "no keywords",
			content:    "some content here",
			keywords:   []string{},
			fileLength: 100,
			minScore:   0.0,
			maxScore:   0.0,
		},
		{
			name:       "no matches",
			content:    "some content here",
			keywords:   []string{"missing", "absent"},
			fileLength: 100,
			minScore:   0.0,
			maxScore:   0.0,
		},
		{
			name:       "partial matches",
			content:    "authentication service handles user login",
			keywords:   []string{"authentication", "missing", "login"},
			fileLength: 100,
			minScore:   0.3, // At least 2/3 keywords matched
			maxScore:   1.0,
		},
		{
			name:       "all matches",
			content:    "authentication service user login",
			keywords:   []string{"authentication", "service", "user", "login"},
			fileLength: 100,
			minScore:   0.5, // All keywords matched
			maxScore:   1.0,
		},
		{
			name:       "length normalized - big file",
			content:    string(make([]byte, 100000)) + " authentication test",
			keywords:   []string{"authentication", "test"},
			fileLength: 100020,
			minScore:   0.0,
			maxScore:   0.5, // Score reduced due to large file size
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateKeywordScore(tt.content, tt.keywords, tt.fileLength)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("calculateKeywordScore() = %v, want between %v and %v",
					score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestCalculatePathScore(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		keywords []string
		expected float32
	}{
		{
			name:     "no keywords",
			filePath: "/path/to/file.go",
			keywords: []string{},
			expected: 0.0,
		},
		{
			name:     "no matches",
			filePath: "/path/to/file.go",
			keywords: []string{"missing", "absent"},
			expected: 0.0,
		},
		{
			name:     "partial match",
			filePath: "/auth/service/login.go",
			keywords: []string{"auth", "missing"},
			expected: 0.5, // 1/2 keywords matched
		},
		{
			name:     "all match",
			filePath: "/auth/service/user.go",
			keywords: []string{"auth", "user"},
			expected: 1.0, // 2/2 keywords matched
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculatePathScore(tt.filePath, tt.keywords)
			if score != tt.expected {
				t.Errorf("calculatePathScore() = %v, want %v", score, tt.expected)
			}
		})
	}
}

func TestCalculateAggregateScore(t *testing.T) {
	tests := []struct {
		name     string
		scores   []float32
		expected float32
	}{
		{
			name:     "empty scores",
			scores:   []float32{},
			expected: 0.0,
		},
		{
			name:     "single score",
			scores:   []float32{0.8},
			expected: 0.4, // 0.8 * 0.5
		},
		{
			name:     "two scores",
			scores:   []float32{0.8, 0.6},
			expected: 0.58, // 0.8*0.5 + 0.6*0.3
		},
		{
			name:     "three scores",
			scores:   []float32{0.8, 0.6, 0.4},
			expected: 0.66, // 0.8*0.5 + 0.6*0.3 + 0.4*0.2
		},
		{
			name:     "more than three scores - only top 3 used",
			scores:   []float32{0.9, 0.8, 0.7, 0.6, 0.5},
			expected: 0.81, // 0.9*0.5 + 0.8*0.3 + 0.7*0.2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateAggregateScore(tt.scores)
			// Allow small floating point error
			if score < tt.expected-0.01 || score > tt.expected+0.01 {
				t.Errorf("calculateAggregateScore() = %v, want %v", score, tt.expected)
			}
		})
	}
}

func TestMin32(t *testing.T) {
	if min32(1.5, 2.5) != 1.5 {
		t.Error("min32(1.5, 2.5) should be 1.5")
	}
	if min32(3.5, 2.5) != 2.5 {
		t.Error("min32(3.5, 2.5) should be 2.5")
	}
}

func TestMax32(t *testing.T) {
	if max32(1.5, 2.5) != 2.5 {
		t.Error("max32(1.5, 2.5) should be 2.5")
	}
	if max32(3.5, 2.5) != 3.5 {
		t.Error("max32(3.5, 2.5) should be 3.5")
	}
}
