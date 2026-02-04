package tui

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DirPicker is a directory tree selector
type DirPicker struct {
	*tview.TreeView
	root     *tview.TreeNode
	rootPath string
	onSelect func(string)
	onCancel func()
}

// NewDirPicker creates a new directory picker starting at the given path
func NewDirPicker(rootPath string) *DirPicker {
	if rootPath == "" {
		rootPath, _ = os.UserHomeDir()
	}

	d := &DirPicker{
		TreeView: tview.NewTreeView(),
		rootPath: rootPath,
	}

	d.root = tview.NewTreeNode(rootPath).SetColor(tcell.ColorYellow)
	d.root.SetReference(rootPath)
	d.SetRoot(d.root)
	d.SetCurrentNode(d.root)

	d.SetTitle(" Select Directory ").SetBorder(true)

	d.populateNode(d.root, rootPath)

	d.SetSelectedFunc(func(node *tview.TreeNode) {
		path := node.GetReference().(string)
		if node.IsExpanded() {
			node.SetExpanded(false)
		} else {
			children := node.GetChildren()
			if len(children) == 0 {
				d.populateNode(node, path)
			}
			node.SetExpanded(true)
		}
	})

	d.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			if d.onCancel != nil {
				d.onCancel()
			}
			return nil
		case tcell.KeyEnter:
			node := d.GetCurrentNode()
			if node != nil && d.onSelect != nil {
				d.onSelect(node.GetReference().(string))
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'h':
				node := d.GetCurrentNode()
				if node != nil && node.IsExpanded() {
					node.SetExpanded(false)
				}
				return nil
			case 'l':
				node := d.GetCurrentNode()
				if node != nil {
					path := node.GetReference().(string)
					if !node.IsExpanded() {
						children := node.GetChildren()
						if len(children) == 0 {
							d.populateNode(node, path)
						}
						node.SetExpanded(true)
					}
				}
				return nil
			case 'q':
				if d.onCancel != nil {
					d.onCancel()
				}
				return nil
			}
		}
		return event
	})

	return d
}

func (d *DirPicker) populateNode(node *tview.TreeNode, path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	var dirs []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() && entry.Name()[0] != '.' {
			dirs = append(dirs, entry)
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name() < dirs[j].Name()
	})

	for _, dir := range dirs {
		childPath := filepath.Join(path, dir.Name())
		child := tview.NewTreeNode(dir.Name()).SetReference(childPath).SetSelectable(true)
		child.SetColor(tcell.ColorGreen)
		node.AddChild(child)
	}
}

// SetOnSelect sets the callback when a directory is selected
func (d *DirPicker) SetOnSelect(fn func(string)) *DirPicker {
	d.onSelect = fn
	return d
}

// SetOnCancel sets the callback when selection is cancelled
func (d *DirPicker) SetOnCancel(fn func()) *DirPicker {
	d.onCancel = fn
	return d
}

// GetSelectedPath returns the currently selected path
func (d *DirPicker) GetSelectedPath() string {
	node := d.GetCurrentNode()
	if node == nil {
		return d.rootPath
	}
	return node.GetReference().(string)
}
