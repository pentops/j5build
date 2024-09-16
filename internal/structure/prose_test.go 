package structure

import "testing"

func TestRemoveMarkdownHeader(t *testing.T) {
	tc := []struct {
		name     string
		data     string
		expected string
	}{
		{
			name:     "empty",
			data:     "",
			expected: "",
		},
		{
			name:     "header and content",
			data:     "# Title\n\nsome content",
			expected: "some content",
		},
		{
			name:     "header with line",
			data:     "Title\n=====\n\nsome content",
			expected: "some content",
		},
		{
			name:     "header only",
			data:     "# Title\n\n",
			expected: "",
		},
		{
			name:     "no header",
			data:     "some content",
			expected: "some content",
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			actual := removeMarkdownHeader(tt.data)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
