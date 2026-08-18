package manager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewInstanceLoggerAtUsesExplicitLogRoot(t *testing.T) {
	root := t.TempDir()
	logger, err := NewInstanceLoggerAt(root, "abc123", 2404)
	if err != nil {
		t.Fatalf("NewInstanceLoggerAt: %v", err)
	}
	if _, err := logger.Write([]byte("test log\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	path := filepath.Join(root, "abc123-2404", "instance.log")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log file was not created at explicit root: %v", err)
	}
}
