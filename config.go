package main

import (
	"fmt"

	"github.com/spf13/viper"
)

var baseConfig = `name: myapp
registry: my.realregistry.com/me
email: user
authfile: ./config.json
server: 0.0.0.0
`

type Config struct {
	Name            string
	Target          string
	Registry        string
	Email           string
	AuthFile        string
	Server          string
	Platform        string
	Volumes         []string
	Hostname        string
	Web             bool
	EnvironmentFile string
	BuildArgFile    string
}

func loadConfig(configKey string) (*Config, error) {
	configName := "lord"
	if configKey != "" {
		configName = fmt.Sprintf("%s.lord", configKey)
	}

	viper.SetConfigName(configName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("target", "")
	viper.SetDefault("platform", "linux/amd64")
	viper.SetDefault("web", false)
	viper.SetDefault("email", "admin@localshot")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	var c Config
	err = viper.Unmarshal(&c)
	if err != nil {
		return nil, err
	}

	fmt.Println("config loaded")

	return &c, nil
}
