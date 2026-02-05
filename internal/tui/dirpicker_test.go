package tui

import (
	"os"
	"testing"
)

func TestNewDirPicker(t *testing.T) {
	picker := NewDirPicker("")

	if picker == nil {
		t.Fatal("NewDirPicker() returned nil")
	}

	home, _ := os.UserHomeDir()
	if picker.rootPath != home {
		t.Errorf("rootPath = %v, want %v", picker.rootPath, home)
	}
}

func TestNewDirPickerWithPath(t *testing.T) {
	picker := NewDirPicker("/tmp")

	if picker == nil {
		t.Fatal("NewDirPicker() returned nil")
	}

	if picker.rootPath != "/tmp" {
		t.Errorf("rootPath = %v, want /tmp", picker.rootPath)
	}
}

func TestDirPickerCallbacks(t *testing.T) {
	picker := NewDirPicker("/tmp")

	var selectCalled bool
	var cancelCalled bool
	var selectedPath string

	picker.SetOnSelect(func(path string) {
		selectCalled = true
		selectedPath = path
	})

	picker.SetOnCancel(func() {
		cancelCalled = true
	})

	// Verify callbacks are set
	if picker.onSelect == nil {
		t.Error("onSelect callback not set")
	}
	if picker.onCancel == nil {
		t.Error("onCancel callback not set")
	}

	// Test cancel callback
	picker.onCancel()
	if !cancelCalled {
		t.Error("onCancel callback not called")
	}

	// Test select callback
	picker.onSelect("/tmp/test")
	if !selectCalled {
		t.Error("onSelect callback not called")
	}
	if selectedPath != "/tmp/test" {
		t.Errorf("selectedPath = %v, want /tmp/test", selectedPath)
	}
}

func TestGetSelectedPath(t *testing.T) {
	picker := NewDirPicker("/tmp")

	path := picker.GetSelectedPath()
	if path != "/tmp" {
		t.Errorf("GetSelectedPath() = %v, want /tmp", path)
	}
}
