package yamlconf

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type PermissionsFile struct {
	Macros      map[string]string `yaml:"macros"`
	Permissions PermissionRules   `yaml:"permissions"`
}

type PermissionRules struct {
	Allow ToolRules `yaml:"allow"`
	Deny  ToolRules `yaml:"deny"`
}

type ToolRules struct {
	Bash []string
}

func (tr *ToolRules) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping, got kind %d", value.Kind)
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value == "bash" {
			patterns, err := flattenNode("", value.Content[i+1])
			if err != nil {
				return err
			}
			tr.Bash = patterns
			return nil
		}
	}
	return nil
}

// flattenNode recursively flattens a yaml.Node into a list of string patterns,
// prepending prefix to each result. This handles mixed lists of scalars and
// nested dicts, where dict keys build up a command prefix.
func flattenNode(prefix string, node *yaml.Node) ([]string, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return []string{join(prefix, node.Value)}, nil
	case yaml.SequenceNode:
		var results []string
		for _, item := range node.Content {
			got, err := flattenNode(prefix, item)
			if err != nil {
				return nil, err
			}
			results = append(results, got...)
		}
		return results, nil
	case yaml.MappingNode:
		var results []string
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			got, err := flattenNode(join(prefix, key), node.Content[i+1])
			if err != nil {
				return nil, err
			}
			results = append(results, got...)
		}
		return results, nil
	default:
		return nil, fmt.Errorf("unexpected YAML node kind %d", node.Kind)
	}
}

func join(prefix, s string) string {
	return strings.TrimSpace(prefix + " " + s)
}

func Load(path string) (*PermissionsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var pf PermissionsFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &pf, nil
}
