package pathutil

import (
	"strings"
	"testing"
)

func TestExpand(t *testing.T) {
	t.Run("tilde prefix expands", func(t *testing.T) {
		result, err := Expand("~/foo/bar")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.HasPrefix(result, "~") {
			t.Errorf("tilde was not expanded: %q", result)
		}
		if !strings.HasSuffix(result, "/foo/bar") {
			t.Errorf("expected suffix /foo/bar, got %q", result)
		}
	})

	t.Run("absolute path unchanged", func(t *testing.T) {
		path := "/abs/path/to/project"
		result, err := Expand(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != path {
			t.Errorf("expected %q, got %q", path, result)
		}
	})

	t.Run("relative path resolves from home", func(t *testing.T) {
		result, err := Expand("Dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(result, "/Dev") {
			t.Errorf("expected suffix /Dev, got %q", result)
		}
		if strings.HasPrefix(result, "Dev") {
			t.Errorf("relative path was not resolved: %q", result)
		}
	})

	t.Run("dot-slash relative path resolves from home", func(t *testing.T) {
		result, err := Expand("./Dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(result, "/Dev") {
			t.Errorf("expected suffix /Dev, got %q", result)
		}
	})
}
