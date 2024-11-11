package main

import (
	"fmt"

	"github.com/spf13/viper"
)

var baseConfig = `name: myapp
registry: my.realregistry.com/me
username: user
password: password
server: 0.0.0.0
`

type Config struct {
	Name     string
	Registry string
	Username string
	Password string
	Server   string
	Platform string
	Volumes  []string
}

func loadConfig() (*Config, error) {
	viper.SetConfigName("lord")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("platform", "linux/amd64")

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
