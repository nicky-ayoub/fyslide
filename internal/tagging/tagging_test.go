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
			expected: nil,
		},
		{
			name:     "Only delimiters",
			input:    ",,,",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTags(tt.input)
			// We use DeepEqual to compare slices, which handles order and values.
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("NormalizeTags(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
