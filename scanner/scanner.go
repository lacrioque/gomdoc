// Package scanner provides functionality for discovering markdown files
// and building a tree structure for the index page.
package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileEntry represents a discovered markdown file.
type FileEntry struct {
	// RelPath is the path relative to the base directory.
	RelPath string
	// Name is the file name without extension.
	Name string
}

// TreeNode represents a node in the file tree (file or directory).
type TreeNode struct {
	Name     string
	Path     string // URL path for files, empty for directories
	IsDir    bool
	Children []*TreeNode
}

// ScanDirectory recursively finds all markdown files in the given root directory.
func ScanDirectory(root string) ([]FileEntry, error) {
	var entries []FileEntry

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and files
		if strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process markdown files
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		name := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
		entries = append(entries, FileEntry{
			RelPath: relPath,
			Name:    name,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort entries alphabetically by path
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelPath < entries[j].RelPath
	})

	return entries, nil
}

// BuildTree constructs a tree structure from a list of file entries.
func BuildTree(entries []FileEntry) *TreeNode {
	root := &TreeNode{
		Name:     "root",
		IsDir:    true,
		Children: make([]*TreeNode, 0),
	}

	for _, entry := range entries {
		parts := strings.Split(filepath.ToSlash(entry.RelPath), "/")
		insertNode(root, parts, entry)
	}

	sortTree(root)
	return root
}

// insertNode adds a file entry to the tree, creating intermediate directories as needed.
func insertNode(parent *TreeNode, parts []string, entry FileEntry) {
	if len(parts) == 0 {
		return
	}

	// Find or create the child node
	var child *TreeNode
	for _, c := range parent.Children {
		if c.Name == parts[0] {
			child = c
			break
		}
	}

	if child == nil {
		isFile := len(parts) == 1
		child = &TreeNode{
			Name:     parts[0],
			IsDir:    !isFile,
			Children: make([]*TreeNode, 0),
		}
		if isFile {
			// Convert path to URL (remove .md extension)
			urlPath := strings.TrimSuffix(entry.RelPath, ".md")
			urlPath = strings.TrimSuffix(urlPath, ".MD")
			child.Path = "/" + filepath.ToSlash(urlPath)
		}
		parent.Children = append(parent.Children, child)
	}

	// Recurse for directories
	if len(parts) > 1 {
		insertNode(child, parts[1:], entry)
	}
}

// sortTree recursively sorts the tree nodes (directories first, then alphabetically).
func sortTree(node *TreeNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		// Directories first
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		// Then alphabetically
		return strings.ToLower(node.Children[i].Name) < strings.ToLower(node.Children[j].Name)
	})

	for _, child := range node.Children {
		if child.IsDir {
			sortTree(child)
		}
	}
}

// RenderTree generates an HTML tree view from the tree structure.
func RenderTree(node *TreeNode) string {
	var sb strings.Builder
	renderTreeNode(&sb, node, 0)
	return sb.String()
}

// renderTreeNode recursively renders a tree node as HTML.
func renderTreeNode(sb *strings.Builder, node *TreeNode, depth int) {
	// Skip the root node itself, just render its children
	if depth == 0 {
		sb.WriteString("<ul class=\"file-tree\">\n")
		for _, child := range node.Children {
			renderTreeNode(sb, child, depth+1)
		}
		sb.WriteString("</ul>\n")
		return
	}

	sb.WriteString("<li>")
	if node.IsDir {
		sb.WriteString("<span class=\"folder\">")
		sb.WriteString(escapeHTML(node.Name))
		sb.WriteString("</span>\n")
		if len(node.Children) > 0 {
			sb.WriteString("<ul>\n")
			for _, child := range node.Children {
				renderTreeNode(sb, child, depth+1)
			}
			sb.WriteString("</ul>\n")
		}
	}
	if !node.IsDir {
		sb.WriteString("<a href=\"")
		sb.WriteString(node.Path)
		sb.WriteString("\" class=\"file\">")
		sb.WriteString(escapeHTML(node.Name))
		sb.WriteString("</a>")
	}
	sb.WriteString("</li>\n")
}

// escapeHTML escapes special HTML characters.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
