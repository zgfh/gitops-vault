package yamledit

import (
	"bytes"
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

// MarshalNode marshals a yaml.Node tree to YAML bytes while preserving
// the literal block style for values containing newlines.
func MarshalNode(doc *yaml.Node) ([]byte, error) {
	preserveStyles(doc)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	enc.Close()
	return buf.Bytes(), nil
}

// preserveStyles ensures that scalar nodes with multi-line values
// use block scalar style (|) for proper YAML output, regardless of
// the current Style (which may be DoubleQuotedStyle if the value was
// previously a single-line placeholder in quotes).
func preserveStyles(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode && strings.Contains(node.Value, "\n") {
		node.Style = yaml.LiteralStyle
	}
	for _, child := range node.Content {
		preserveStyles(child)
	}
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
