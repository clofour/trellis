package spec

import (
	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
)

// ParseYAML decodes a YAML job specification.
func ParseYAML(raw []byte) (*JobSpec, error) {
	var data map[string]interface{}
	err := yaml.Unmarshal(raw, &data)
	if err != nil {
		return nil, err
	}
	normalizeLegacyAPIAccess(data)

	var job *JobSpec
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused:      true,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result:  &job,
		TagName: "yaml",
	})
	if err != nil {
		return nil, err
	}
	return job, decoder.Decode(data)
}

func normalizeLegacyAPIAccess(data map[string]interface{}) {
	groups, ok := data["task_groups"].([]interface{})
	if !ok {
		return
	}
	for _, rawGroup := range groups {
		group, ok := rawGroup.(map[string]interface{})
		if !ok {
			continue
		}
		legacy, ok := group["api_access"].(bool)
		if !ok {
			continue
		}
		if legacy {
			group["api_access"] = string(APIAccessNamespace)
		} else {
			delete(group, "api_access")
		}
	}
}
