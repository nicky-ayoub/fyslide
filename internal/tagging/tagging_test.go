package tagging

import (
	"reflect"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple comma separated",
			input:    "tag1,tag2,tag3",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "Mixed delimiters",
			input:    "tag1.tag2;tag3+tag4",
			expected: []string{"tag1", "tag2", "tag3", "tag4"},
		},
		{
			name:     "Whitespace handling",
			input:    "  tag1 ,  tag2  ",
			expected: []string{"tag1", "tag2"},
		},
		{
			name:     "Case normalization",
			input:    "Tag1, TAG2, taG3",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "Duplicates removal",
			input:    "tag1, tag1, TAG1",
			expected: []string{"tag1"},
		},
		{
			name:     "Empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "Only delimiters",
			input:    ",,,",
			expected: []string{},
		},
		{
			name:     "Multi-word tags",
			input:    "new york, big apple",
			expected: []string{"new york", "big apple"},
		},
		{
			name:     "Tags with quotes",
			input:    `"tag1", 'tag2'`,
			expected: []string{"tag1", "tag2"},
		},
		{
			name:     "Tags with quotes and spaces",
			input:    `  " spaced tag " , 'another'  `,
			expected: []string{"spaced tag", "another"},
		},
		{
			name:     "Empty quoted tags",
			input:    `"tag1", '', ""`,
			expected: []string{"tag1"},
		},
		{
			name:     "Mixed multi-word and single",
			input:    "photo, summer vacation, beach",
			expected: []string{"photo", "summer vacation", "beach"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTags(tt.input)
			// An empty slice is the desired outcome for no tags, not nil.
			if len(got) == 0 && len(tt.expected) == 0 {
				return // Both are empty, treat as equal.
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("NormalizeTags(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
