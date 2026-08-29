package spec

import (
	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
)

func ParseYAML(raw []byte) (*JobSpec, error) {
	var data map[string]interface{}
	err := yaml.Unmarshal(raw, &data)
	if err != nil {
		return nil, err
	}

	var job *JobSpec
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
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
