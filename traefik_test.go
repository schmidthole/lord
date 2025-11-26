package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateTraefikConfig(t *testing.T) {
	t.Run("basic config without timeouts", func(t *testing.T) {
		email := "test@example.com"
		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  -1,
			WriteTimeout: -1,
			IdleTimeout:  -1,
		}

		yamlStr, err := createTraefikConfig(email, webAdvancedConfig)
		assert.NoError(t, err)
		assert.Contains(t, yamlStr, "address: :80")
		assert.Contains(t, yamlStr, "address: :443")
		assert.Contains(t, yamlStr, "email: test@example.com")
		assert.Contains(t, yamlStr, "exposedByDefault: false")
		assert.NotContains(t, yamlStr, "respondingTimeouts")
	})

	t.Run("config with all timeouts set", func(t *testing.T) {
		email := "test@example.com"
		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  60,
			WriteTimeout: 120,
			IdleTimeout:  180,
		}

		yamlStr, err := createTraefikConfig(email, webAdvancedConfig)
		assert.NoError(t, err)
		assert.Contains(t, yamlStr, "readTimeout: 60s")
		assert.Contains(t, yamlStr, "writeTimeout: 120s")
		assert.Contains(t, yamlStr, "idleTimeout: 180s")
	})

	t.Run("config with only read timeout set", func(t *testing.T) {
		email := "test@example.com"
		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  60,
			WriteTimeout: -1,
			IdleTimeout:  -1,
		}

		yamlStr, err := createTraefikConfig(email, webAdvancedConfig)
		assert.NoError(t, err)
		assert.Contains(t, yamlStr, "readTimeout: 60s")
		assert.NotContains(t, yamlStr, "writeTimeout")
		assert.NotContains(t, yamlStr, "idleTimeout")
	})

	t.Run("config with zero timeout (unlimited)", func(t *testing.T) {
		email := "test@example.com"
		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  0,
			WriteTimeout: 0,
			IdleTimeout:  180,
		}

		yamlStr, err := createTraefikConfig(email, webAdvancedConfig)
		assert.NoError(t, err)
		assert.Contains(t, yamlStr, "readTimeout: 0s")
		assert.Contains(t, yamlStr, "writeTimeout: 0s")
		assert.Contains(t, yamlStr, "idleTimeout: 180s")
	})
}

func TestReadTraefikConfig(t *testing.T) {
	t.Run("valid yaml", func(t *testing.T) {
		yamlStr := `
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"
    transport:
      respondingTimeouts:
        readTimeout: 60s
        writeTimeout: 120s
certificatesResolvers:
  theresolver:
    acme:
      email: test@example.com
      storage: acme.json
      httpChallenge:
        entryPoint: web
providers:
  docker:
    exposedByDefault: false
`

		config, err := readTraefikConfig(yamlStr)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, ":80", config.EntryPoints["web"].Address)
		assert.Equal(t, ":443", config.EntryPoints["websecure"].Address)
		assert.Equal(t, "60s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.ReadTimeout)
		assert.Equal(t, "120s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.WriteTimeout)
		assert.Equal(t, "test@example.com", config.CertificatesResolvers["theresolver"].ACME.Email)
		assert.False(t, config.Providers.Docker.ExposedByDefault)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		yamlStr := `
invalid: yaml: structure:
  - this is not valid
  nested::double colon
`
		config, err := readTraefikConfig(yamlStr)
		assert.Error(t, err)
		assert.Nil(t, config)
	})

	t.Run("empty yaml", func(t *testing.T) {
		yamlStr := ""
		config, err := readTraefikConfig(yamlStr)
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})
}

func TestMaybeUpdateTraefikAdvancedWebConfig(t *testing.T) {
	t.Run("no existing timeouts, add new ones", func(t *testing.T) {
		config := &TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"websecure": {
					Address: ":443",
				},
			},
		}

		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  60,
			WriteTimeout: 120,
			IdleTimeout:  180,
		}

		updated := maybeUpdateTraefikAdvancedWebConfig(config, webAdvancedConfig)
		assert.True(t, updated)
		assert.NotNil(t, config.EntryPoints["websecure"].Transport)
		assert.NotNil(t, config.EntryPoints["websecure"].Transport.RespondingTimeouts)
		assert.Equal(t, "60s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.ReadTimeout)
		assert.Equal(t, "120s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.WriteTimeout)
		assert.Equal(t, "180s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.IdleTimeout)
	})

	t.Run("existing timeouts are lower, update them", func(t *testing.T) {
		config := &TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"websecure": {
					Address: ":443",
					Transport: &EntryPointTransport{
						RespondingTimeouts: &RespondingTimeouts{
							ReadTimeout:  "30s",
							WriteTimeout: "60s",
							IdleTimeout:  "90s",
						},
					},
				},
			},
		}

		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  60,
			WriteTimeout: 120,
			IdleTimeout:  180,
		}

		updated := maybeUpdateTraefikAdvancedWebConfig(config, webAdvancedConfig)
		assert.True(t, updated)
		assert.Equal(t, "60s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.ReadTimeout)
		assert.Equal(t, "120s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.WriteTimeout)
		assert.Equal(t, "180s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.IdleTimeout)
	})

	t.Run("existing timeouts are higher, keep them", func(t *testing.T) {
		config := &TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"websecure": {
					Address: ":443",
					Transport: &EntryPointTransport{
						RespondingTimeouts: &RespondingTimeouts{
							ReadTimeout:  "120s",
							WriteTimeout: "240s",
							IdleTimeout:  "360s",
						},
					},
				},
			},
		}

		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  60,
			WriteTimeout: 120,
			IdleTimeout:  180,
		}

		updated := maybeUpdateTraefikAdvancedWebConfig(config, webAdvancedConfig)
		assert.False(t, updated)
		assert.Equal(t, "120s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.ReadTimeout)
		assert.Equal(t, "240s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.WriteTimeout)
		assert.Equal(t, "360s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.IdleTimeout)
	})

	t.Run("existing timeout is 0 (unlimited), keep it", func(t *testing.T) {
		config := &TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"websecure": {
					Address: ":443",
					Transport: &EntryPointTransport{
						RespondingTimeouts: &RespondingTimeouts{
							ReadTimeout:  "0s",
							WriteTimeout: "60s",
							IdleTimeout:  "180s",
						},
					},
				},
			},
		}

		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  120,
			WriteTimeout: 120,
			IdleTimeout:  180,
		}

		updated := maybeUpdateTraefikAdvancedWebConfig(config, webAdvancedConfig)
		assert.True(t, updated) // only writetimeout updates
		assert.Equal(t, "0s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.ReadTimeout)
		assert.Equal(t, "120s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.WriteTimeout)
		assert.Equal(t, "180s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.IdleTimeout)
	})

	t.Run("new timeout is 0 (unlimited), update existing", func(t *testing.T) {
		config := &TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"websecure": {
					Address: ":443",
					Transport: &EntryPointTransport{
						RespondingTimeouts: &RespondingTimeouts{
							ReadTimeout:  "60s",
							WriteTimeout: "120s",
							IdleTimeout:  "180s",
						},
					},
				},
			},
		}

		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  0,
			WriteTimeout: -1,
			IdleTimeout:  -1,
		}

		updated := maybeUpdateTraefikAdvancedWebConfig(config, webAdvancedConfig)
		assert.True(t, updated)
		assert.Equal(t, "0s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.ReadTimeout)
		assert.Equal(t, "120s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.WriteTimeout)
		assert.Equal(t, "180s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.IdleTimeout)
	})

	t.Run("new config has -1 (not set), keep existing", func(t *testing.T) {
		config := &TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"websecure": {
					Address: ":443",
					Transport: &EntryPointTransport{
						RespondingTimeouts: &RespondingTimeouts{
							ReadTimeout:  "60s",
							WriteTimeout: "120s",
							IdleTimeout:  "180s",
						},
					},
				},
			},
		}

		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  -1,
			WriteTimeout: -1,
			IdleTimeout:  -1,
		}

		updated := maybeUpdateTraefikAdvancedWebConfig(config, webAdvancedConfig)
		assert.False(t, updated)
		assert.Equal(t, "60s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.ReadTimeout)
		assert.Equal(t, "120s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.WriteTimeout)
		assert.Equal(t, "180s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.IdleTimeout)
	})

	t.Run("mixed scenario - some update, some keep", func(t *testing.T) {
		config := &TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"websecure": {
					Address: ":443",
					Transport: &EntryPointTransport{
						RespondingTimeouts: &RespondingTimeouts{
							ReadTimeout:  "30s",
							WriteTimeout: "240s",
							IdleTimeout:  "",
						},
					},
				},
			},
		}

		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  60,
			WriteTimeout: 120,
			IdleTimeout:  180,
		}

		updated := maybeUpdateTraefikAdvancedWebConfig(config, webAdvancedConfig)
		assert.True(t, updated)
		assert.Equal(t, "60s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.ReadTimeout)   // updated from 30s to 60s
		assert.Equal(t, "240s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.WriteTimeout) // kept at 240s (higher)
		assert.Equal(t, "180s", config.EntryPoints["websecure"].Transport.RespondingTimeouts.IdleTimeout)  // added (was empty)
	})

	t.Run("no websecure entrypoint, return false", func(t *testing.T) {
		config := &TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"web": {
					Address: ":80",
				},
			},
		}

		webAdvancedConfig := WebAdvancedConfig{
			ReadTimeout:  60,
			WriteTimeout: 120,
			IdleTimeout:  180,
		}

		updated := maybeUpdateTraefikAdvancedWebConfig(config, webAdvancedConfig)
		assert.False(t, updated)
	})
}

func TestTraefikConfigSerialize(t *testing.T) {
	t.Run("serialize basic config", func(t *testing.T) {
		config := TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"web": {
					Address: ":80",
				},
				"websecure": {
					Address: ":443",
				},
			},
			CertificatesResolvers: map[string]CertificateResolver{
				"theresolver": {
					ACME: ACMEConfig{
						Email:   "test@example.com",
						Storage: "acme.json",
						HTTPChallenge: HTTPChallenge{
							EntryPoint: "web",
						},
					},
				},
			},
			Providers: Providers{
				Docker: DockerProvider{
					ExposedByDefault: false,
				},
			},
		}

		yamlStr, err := config.serialize()
		assert.NoError(t, err)
		assert.Contains(t, yamlStr, "address: :80")
		assert.Contains(t, yamlStr, "address: :443")
		assert.Contains(t, yamlStr, "email: test@example.com")
	})

	t.Run("serialize config with timeouts", func(t *testing.T) {
		config := TraefikConfig{
			EntryPoints: map[string]EntryPoint{
				"websecure": {
					Address: ":443",
					Transport: &EntryPointTransport{
						RespondingTimeouts: &RespondingTimeouts{
							ReadTimeout:  "60s",
							WriteTimeout: "120s",
							IdleTimeout:  "180s",
						},
					},
				},
			},
			CertificatesResolvers: map[string]CertificateResolver{
				"theresolver": {
					ACME: ACMEConfig{
						Email:   "test@example.com",
						Storage: "acme.json",
						HTTPChallenge: HTTPChallenge{
							EntryPoint: "web",
						},
					},
				},
			},
			Providers: Providers{
				Docker: DockerProvider{
					ExposedByDefault: false,
				},
			},
		}

		yamlStr, err := config.serialize()
		assert.NoError(t, err)
		assert.Contains(t, yamlStr, "readTimeout: 60s")
		assert.Contains(t, yamlStr, "writeTimeout: 120s")
		assert.Contains(t, yamlStr, "idleTimeout: 180s")
	})
}

func TestTraefikNeedsAdvancedConfig(t *testing.T) {
	t.Run("no advanced config needed", func(t *testing.T) {
		r := &remote{
			config: &Config{
				WebAdvancedConfig: WebAdvancedConfig{
					ReadTimeout:  -1,
					WriteTimeout: -1,
					IdleTimeout:  -1,
				},
			},
		}

		assert.False(t, r.traefikNeedsAdvancedConfig())
	})

	t.Run("read timeout set", func(t *testing.T) {
		r := &remote{
			config: &Config{
				WebAdvancedConfig: WebAdvancedConfig{
					ReadTimeout:  60,
					WriteTimeout: -1,
					IdleTimeout:  -1,
				},
			},
		}

		assert.True(t, r.traefikNeedsAdvancedConfig())
	})

	t.Run("write timeout set", func(t *testing.T) {
		r := &remote{
			config: &Config{
				WebAdvancedConfig: WebAdvancedConfig{
					ReadTimeout:  -1,
					WriteTimeout: 120,
					IdleTimeout:  -1,
				},
			},
		}

		assert.True(t, r.traefikNeedsAdvancedConfig())
	})

	t.Run("idle timeout set", func(t *testing.T) {
		r := &remote{
			config: &Config{
				WebAdvancedConfig: WebAdvancedConfig{
					ReadTimeout:  -1,
					WriteTimeout: -1,
					IdleTimeout:  180,
				},
			},
		}

		assert.True(t, r.traefikNeedsAdvancedConfig())
	})

	t.Run("all timeouts set", func(t *testing.T) {
		r := &remote{
			config: &Config{
				WebAdvancedConfig: WebAdvancedConfig{
					ReadTimeout:  60,
					WriteTimeout: 120,
					IdleTimeout:  180,
				},
			},
		}

		assert.True(t, r.traefikNeedsAdvancedConfig())
	})

	t.Run("zero timeout (unlimited) should be considered set", func(t *testing.T) {
		r := &remote{
			config: &Config{
				WebAdvancedConfig: WebAdvancedConfig{
					ReadTimeout:  0,
					WriteTimeout: -1,
					IdleTimeout:  -1,
				},
			},
		}

		assert.True(t, r.traefikNeedsAdvancedConfig())
	})
}