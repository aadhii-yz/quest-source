package bundle

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Definition is the subset of course-definition.yml the builder needs to
// validate a course and populate the index. The bundle ships the full file.
type Definition struct {
	Slug               string `yaml:"slug"`
	Name               string `yaml:"name"`
	ShortDescriptionMD string `yaml:"short_description_md"`
	ReleaseStatus      string `yaml:"release_status"`
	Languages          []struct {
		Slug string `yaml:"slug"`
	} `yaml:"languages"`
	Stages []struct {
		Slug string `yaml:"slug"`
	} `yaml:"stages"`
}

// ParseDefinition decodes course-definition.yml. Unknown fields are ignored,
// mirroring the app's tolerant wire-format handling.
func ParseDefinition(data []byte) (Definition, error) {
	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return Definition{}, fmt.Errorf("parse course-definition.yml: %w", err)
	}
	return def, nil
}

// Languages returns the language slugs in upstream order.
func (d Definition) LanguagesList() []string {
	out := make([]string, 0, len(d.Languages))
	for _, l := range d.Languages {
		out = append(out, l.Slug)
	}
	return out
}

// StageCount returns the number of stages in the definition.
func (d Definition) StageCount() int { return len(d.Stages) }

// ValidateDefinition checks that a definition matches its pin and is complete
// enough to bundle.
func ValidateDefinition(pin Pin, def Definition) error {
	if def.Slug == "" {
		return fmt.Errorf("%s: course-definition.yml has no slug", pin.Slug)
	}
	if def.Slug != pin.Slug {
		return fmt.Errorf("%s: course-definition.yml slug %q does not match pin", pin.Slug, def.Slug)
	}
	if def.Name == "" {
		return fmt.Errorf("%s: course-definition.yml has no name", pin.Slug)
	}
	if len(def.Languages) == 0 {
		return fmt.Errorf("%s: course-definition.yml has no languages", pin.Slug)
	}
	if len(def.Stages) == 0 {
		return fmt.Errorf("%s: course-definition.yml has no stages", pin.Slug)
	}
	return nil
}
