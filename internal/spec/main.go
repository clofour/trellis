package spec

import (
	"github.com/clofour/trellis/internal/models"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
)

func ParseSpec(raw []byte) ([]models.Job, error) {
	var data map[string]interface{}
	err := yaml.Unmarshal(raw, &data)
	if err != nil {
		return nil, err
	}

	var job *models.Job
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
