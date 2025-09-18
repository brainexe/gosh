package pkg

import (
	"os"
	"strings"
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

func TestAutocompleteCompleter(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "gosh_test_completer")
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

	// Create test files
	testFiles := []string{"upload.txt", "script.sh"}
	for _, file := range testFiles {
		err := os.WriteFile(file, []byte("test"), 0o600)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	tests := []struct {
		name     string
		line     string
		hosts    []string
		user     string
		expected []string
	}{
		{
			name:     "empty line returns empty",
			line:     "",
			hosts:    []string{"host1"},
			user:     "user",
			expected: []string{},
		},
		{
			name:     ":upload without filename shows all files",
			line:     ":upload ",
			hosts:    []string{"host1"},
			user:     "user",
			expected: []string{"upload.txt", "script.sh"},
		},
		{
			name:     ":upload with partial filename filters results",
			line:     ":upload up",
			hosts:    []string{"host1"},
			user:     "user",
			expected: []string{"upload.txt"},
		},
		{
			name:     "internal command completion for :up",
			line:     ":up",
			hosts:    []string{"host1"},
			user:     "user",
			expected: []string{":upload "},
		},
		{
			name:     "no hosts returns empty for SSH completion",
			line:     "ls",
			hosts:    []string{},
			user:     "user",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Completer(tt.line, tt.hosts, tt.user)

			// For file completions, we can't predict exact SSH completions
			// so we just check that we get some result or empty as expected
			if len(tt.expected) == 0 {
				if len(result) != 0 {
					t.Errorf("Expected empty result, got %v", result)
				}
				return
			}

			// For file-based completions, check that expected files are present
			if strings.HasPrefix(tt.line, ":upload") {
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
			}
		})
	}
}

func TestAutocompleteCompleter_InternalCommands(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "colon only matches upload command",
			line:     ":",
			expected: []string{":upload "},
		},
		{
			name:     ":u matches :upload",
			line:     ":u",
			expected: []string{":upload "},
		},
		{
			name:     ":upload matches itself",
			line:     ":upload",
			expected: []string{":upload "},
		},
		{
			name:     ":nonexistent returns empty",
			line:     ":nonexistent",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Completer(tt.line, []string{"host1"}, "user")

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d completions, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, expected := range tt.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("Expected completion %q at index %d, got %q", expected, i, result[i])
				}
			}
		})
	}
}
