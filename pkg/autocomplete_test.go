package pkg

import (
	"os"
	"testing"
)

func TestAutocompleteGetLocalFileCompletions(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "gosh_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	defer os.Chdir(oldDir)

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Create test files and directories
	testFiles := []string{
		"test.txt",
		"test.go",
		"other.txt",
		"script.sh",
	}
	testDirs := []string{
		"testdir",
		"anotherdir",
	}

	for _, file := range testFiles {
		err := os.WriteFile(file, []byte("test"), 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	for _, dir := range testDirs {
		err := os.Mkdir(dir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create test dir %s: %v", dir, err)
		}
	}

	tests := []struct {
		name     string
		prefix   string
		expected []string
	}{
		{
			name:     "empty prefix returns all files and dirs with trailing slash for dirs",
			prefix:   "",
			expected: []string{"test.txt", "test.go", "other.txt", "script.sh", "testdir/", "anotherdir/"},
		},
		{
			name:     "prefix 'test' matches test files and testdir",
			prefix:   "test",
			expected: []string{"test.txt", "test.go", "testdir/"},
		},
		{
			name:     "prefix 'other' matches other.txt",
			prefix:   "other",
			expected: []string{"other.txt"},
		},
		{
			name:     "prefix 'nonexistent' returns empty",
			prefix:   "nonexistent",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getLocalFileCompletions(tt.prefix)

			// Check that all expected completions are present
			for _, expected := range tt.expected {
				found := false
				for _, actual := range result {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected completion %q not found in result %v", expected, result)
				}
			}

			// Check that result doesn't contain unexpected items
			for _, actual := range result {
				found := false
				for _, expected := range tt.expected {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Unexpected completion %q found in result", actual)
				}
			}
		})
	}
}
