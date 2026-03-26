package bot

import (
	"strings"
	"testing"
)

func TestSplitMessage(t *testing.T) {
	t.Run("short message no split", func(t *testing.T) {
		msg := "hello world"
		parts := splitMessage(msg, 4096)
		if len(parts) != 1 {
			t.Fatalf("expected 1 part, got %d", len(parts))
		}
		if parts[0] != msg {
			t.Errorf("expected %q, got %q", msg, parts[0])
		}
	})

	t.Run("long message splits at newline", func(t *testing.T) {
		// Build a message that exceeds maxLen with a newline near the boundary.
		line1 := strings.Repeat("a", 100) + "\n"
		line2 := strings.Repeat("b", 100)
		msg := line1 + line2
		parts := splitMessage(msg, 150)
		if len(parts) != 2 {
			t.Fatalf("expected 2 parts, got %d: %v", len(parts), parts)
		}
		// First part should be the 100 'a's (without the trailing newline).
		if parts[0] != strings.Repeat("a", 100) {
			t.Errorf("unexpected first part: %q", parts[0])
		}
		if parts[1] != line2 {
			t.Errorf("unexpected second part: %q", parts[1])
		}
	})

	t.Run("long message no newline splits at maxLen", func(t *testing.T) {
		msg := strings.Repeat("x", 200)
		parts := splitMessage(msg, 100)
		if len(parts) != 2 {
			t.Fatalf("expected 2 parts, got %d", len(parts))
		}
		if parts[0] != strings.Repeat("x", 100) {
			t.Errorf("unexpected first part length %d", len(parts[0]))
		}
		if parts[1] != strings.Repeat("x", 100) {
			t.Errorf("unexpected second part length %d", len(parts[1]))
		}
	})
}

func TestIsAllowed(t *testing.T) {
	t.Run("empty list allows all", func(t *testing.T) {
		b := &Bot{allowedUsers: []int64{}}
		if !b.isAllowed(12345) {
			t.Error("expected user to be allowed when allowedUsers is empty")
		}
	})

	t.Run("allowed user", func(t *testing.T) {
		b := &Bot{allowedUsers: []int64{111, 222, 333}}
		if !b.isAllowed(222) {
			t.Error("expected user 222 to be allowed")
		}
	})

	t.Run("denied user", func(t *testing.T) {
		b := &Bot{allowedUsers: []int64{111, 222, 333}}
		if b.isAllowed(999) {
			t.Error("expected user 999 to be denied")
		}
	})
}

func TestExpandPath(t *testing.T) {
	t.Run("tilde prefix expands", func(t *testing.T) {
		result, err := expandPath("~/foo/bar")
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
		result, err := expandPath(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != path {
			t.Errorf("expected %q, got %q", path, result)
		}
	})

	t.Run("relative path resolves from home", func(t *testing.T) {
		result, err := expandPath("Dev")
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
		result, err := expandPath("./Dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(result, "/Dev") {
			t.Errorf("expected suffix /Dev, got %q", result)
		}
	})
}

func TestProjectNameRe(t *testing.T) {
	valid := []string{"myapp", "my-app", "my_app123", "ABC", "a1b2c3"}
	for _, name := range valid {
		if !projectNameRe.MatchString(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{"my app", "../hack", "", "hello world", "foo/bar", "foo@bar"}
	for _, name := range invalid {
		if projectNameRe.MatchString(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
