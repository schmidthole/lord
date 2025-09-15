package main

import (
	"fmt"

	"github.com/spf13/viper"
)

var baseConfig = `name: myapp
registry: my.realregistry.com/me
email: user
server: 0.0.0.0
`

type Config struct {
	// name of the application/container, must be unique per remote host (required)
	Name string

	// build target for docker container if needed, passed during the docker build process as --target (optional)
	Target string

	// container registry url to use (required)
	Registry string

	// email to use for tls certificate notifications. set to a dummy value if not supplied (optional)
	Email string

	// auth config.json for registry, will be copied to remote host if provided. must be the same for all containers on a single host (optional)
	AuthFile string

	// ip address of remote host server. the deployment machine must have ssh access (required)
	Server string

	// platform to build containers for, must match remote host. defaults to linux/amd64 (optional)
	Platform string

	// any additional volumes to mount on the remote host, follows docker convention (optional)
	Volumes []string

	// hostname to use for web applications and tls certs (optional, required if web is true)
	Hostname string

	// whether or not the application is a web service. if true, must expose port 80 from the docker container and specify a hostname
	Web bool

	// environment variable file (optional)
	EnvironmentFile string

	// docker build argument file (optional)
	BuildArgFile string

	// private ssh key file path for server connections (optional)
	SshKeyFile string

	// ssh login user for server connections, defaults to root (optional)
	User string
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
	viper.SetDefault("user", "root")

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
