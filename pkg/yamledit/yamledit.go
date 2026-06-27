package yamledit

import (
	"fmt"
	"strings"

	"github.com/zzg/gitops-vault/pkg/scanner"

	"gopkg.in/yaml.v3"
)

// Edit represents a single edit to be applied to a YAML tree.
type Edit struct {
	Node     *yaml.Node      // the scalar node to modify
	OldValue string          // original value
	NewValue string          // replacement (placeholder)
	Finding  *scanner.Finding // associated scan finding
}

// Walk traverses a YAML node tree, applying the visitor to each scalar node.
// The parent nodes are provided for context to build the path.
func Walk(node *yaml.Node, visitor func(node *yaml.Node, path []string, value string) *scanner.Finding) []*scanner.Finding {
	var findings []*scanner.Finding
	walkNode(node, nil, visitor, &findings)
	return findings
}

func walkNode(node *yaml.Node, path []string, visitor func(node *yaml.Node, path []string, value string) *scanner.Finding, findings *[]*scanner.Finding) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			walkNode(child, path, visitor, findings)
		}
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			keyStr := keyNode.Value
			newPath := append(path, keyStr)
			if valNode.Kind == yaml.ScalarNode && valNode.Value != "" {
				f := visitor(valNode, newPath, valNode.Value)
				if f != nil {
					*findings = append(*findings, f)
				}
			}
			walkNode(valNode, newPath, visitor, findings)
		}
	case yaml.SequenceNode:
		for idx, child := range node.Content {
			newPath := append(path, fmt.Sprintf("[%d]", idx))
			if child.Kind == yaml.ScalarNode && child.Value != "" {
				f := visitor(child, newPath, child.Value)
				if f != nil {
					*findings = append(*findings, f)
				}
			}
			walkNode(child, newPath, visitor, findings)
		}
	}
}

// ApplyEdit replaces a node's value with the new value.
func ApplyEdit(edit *Edit) {
	edit.Node.Value = edit.NewValue
}

// KeyFromPath derives a meaningful key name from a YAML path for placeholder generation.
// e.g., ["stringData", "db_password"] -> "DB_PASSWORD"
func KeyFromPath(path []string) string {
	// Walk backwards to find the first non-array-index segment
	for i := len(path) - 1; i >= 0; i-- {
		seg := path[i]
		if !strings.HasPrefix(seg, "[") {
			return strings.ToUpper(seg)
		}
	}
	if len(path) > 0 {
		return strings.ToUpper(strings.Trim(path[len(path)-1], "[]"))
	}
	return "VALUE"
}
