package yamlconf

import (
	"fmt"
	"os"

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
	Bash []string `yaml:"bash"`
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
