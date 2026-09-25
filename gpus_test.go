package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGPUConfig(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(original)
	defer viper.Reset()
	for _, tc := range []struct {
		name, setting, want string
		invalid             bool
	}{
		{"omitted", "", "", false},
		{"all", "gpus: all\n", "all", false},
		{"device", "gpus: device=0\n", "", true},
		{"injection", "gpus: all; touch /tmp/unwanted\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "lord.yml"), []byte("name: test\nserver: localhost\n"+tc.setting), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			viper.Reset()
			config, err := loadConfig("")
			if tc.invalid {
				if err == nil {
					t.Fatal("expected invalid gpu setting")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if config.GPUs != tc.want {
				t.Fatalf("unexpected gpus: %q", config.GPUs)
			}
		})
	}
}

func TestContainerGPUCommand(t *testing.T) {
	config := &Config{WebAdvancedConfig: WebAdvancedConfig{MaxRequestBodyBytes: -1, MaxResponseBodyBytes: -1, MemRequestBodyBytes: -1}}
	remote := &remote{config: config}
	for _, web := range []bool{false, true} {
		baseline := remote.containerRunCommand("test", "image:tag", []string{"/srv/radio:/radio"}, "test.env", web, "radio.example.com")
		if strings.Contains(baseline, "--gpus") {
			t.Fatal("unexpected gpu access")
		}
		config.GPUs = "all"
		actual := remote.containerRunCommand("test", "image:tag", []string{"/srv/radio:/radio"}, "test.env", web, "radio.example.com")
		if strings.Replace(actual, " --gpus all", "", 1) != baseline || strings.Count(actual, " --gpus all") != 1 {
			t.Fatalf("unexpected command: %s", actual)
		}
		config.GPUs = ""
	}
}
