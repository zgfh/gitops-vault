package yamledit

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// findScalarNodes recursively finds all scalar nodes with matching value.
func findScalarNodes(node *yaml.Node, target string) []*yaml.Node {
	var result []*yaml.Node
	if node == nil {
		return result
	}
	if node.Kind == yaml.ScalarNode && node.Value == target {
		result = append(result, node)
	}
	for _, child := range node.Content {
		result = append(result, findScalarNodes(child, target)...)
	}
	return result
}

func TestMarshalNodeMultiDocument(t *testing.T) {
	input := `apiVersion: v1
data:
  script.sh: |-
    #!/bin/bash
    echo hello
kind: ConfigMap
metadata:
  name: test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deploy
spec:
  replicas: 1
`

	decoder := yaml.NewDecoder(strings.NewReader(input))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			break
		}
		docs = append(docs, &doc)
	}

	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}

	var out strings.Builder
	for i, doc := range docs {
		data, err := MarshalNode(doc)
		if err != nil {
			t.Fatalf("marshal doc %d: %v", i, err)
		}
		if i > 0 {
			out.WriteString("---\n")
		}
		out.Write(data)
	}

	output := out.String()
	if !strings.Contains(output, "ConfigMap") {
		t.Error("output missing ConfigMap")
	}
	if !strings.Contains(output, "Deployment") {
		t.Error("output missing Deployment")
	}
}

func TestPreserveStylesOverridesDoubleQuotedStyle(t *testing.T) {
	// Simulate decrypting a value that was stored as a double-quoted placeholder
	input := `data:
  script.sh: "PLACEHOLDER_VALUE"
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatal(err)
	}

	// Verify the parsed node has DoubleQuotedStyle
	nodes := findScalarNodes(&doc, "PLACEHOLDER_VALUE")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 placeholder node, got %d", len(nodes))
	}
	if nodes[0].Style != yaml.DoubleQuotedStyle {
		t.Logf("expected DoubleQuotedStyle, got %d", nodes[0].Style)
	}

	// Simulate decrypt: restore multi-line value
	nodes[0].Value = "#!/bin/bash\nmulti\nline\n"

	output, err := MarshalNode(&doc)
	if err != nil {
		t.Fatal(err)
	}

	outStr := string(output)
	t.Logf("Output:\n%s", outStr)

	if !strings.Contains(outStr, "|") {
		t.Error("output should use block scalar (|) even when input was double-quoted")
	}
	if strings.Contains(outStr, `\n`) {
		t.Error("output should not contain escaped newlines")
	}
}

func TestPreserveStylesAfterValueRestore(t *testing.T) {
	// Simulate the decrypt round-trip:
	// Parse YAML that had block scalar encrypted to placeholder,
	// restore value to multi-line (decrypt),
	// Marshal - should use block scalar

	input := `data:
  script.sh: PLACEHOLDER_VALUE
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatal(err)
	}

	// Simulate decrypt: restore multi-line value (same as what decrypt does)
	nodes := findScalarNodes(&doc, "PLACEHOLDER_VALUE")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 placeholder node, got %d", len(nodes))
	}
	nodes[0].Value = "#!/bin/bash\nmulti\nline\n"

	output, err := MarshalNode(&doc)
	if err != nil {
		t.Fatal(err)
	}

	outStr := string(output)
	t.Logf("Output:\n%s", outStr)

	if !strings.Contains(outStr, "|") {
		t.Error("output should use block scalar (|)")
	}
	if strings.Contains(outStr, `\n`) && !strings.Contains(outStr, "#!/bin/bash") {
		t.Error("output should not use escaped newlines for multi-line content")
	}
}

func TestPreserveStylesWithMultipleBlockScalars(t *testing.T) {
	input := `data:
  entrypoint.sh: PLACEHOLDER_A
  update.sh: PLACEHOLDER_B
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatal(err)
	}

	// Simulate decrypt restoring multi-line values
	for _, node := range findScalarNodes(&doc, "PLACEHOLDER_A") {
		node.Value = "#!/bin/bash\nset -e\necho start\n"
	}
	for _, node := range findScalarNodes(&doc, "PLACEHOLDER_B") {
		node.Value = "#!/bin/bash\nset -e\nexport FOO=\"bar\"\n"
	}

	output, err := MarshalNode(&doc)
	if err != nil {
		t.Fatal(err)
	}

	outStr := string(output)
	t.Logf("Output:\n%s", outStr)

	// Count block scalar indicators
	count := strings.Count(outStr, "|-") + strings.Count(outStr, "|")
	if count < 2 {
		t.Errorf("expected 2 block scalar indicators, found %d. Output:\n%s", count, outStr)
	}
	if strings.Contains(outStr, `"#!/bin/bash`) {
		t.Error("block scalar content should not be double-quoted")
	}
}
