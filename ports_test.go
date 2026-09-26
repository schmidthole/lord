package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidatePortMapping(t *testing.T) {
	for _, mapping := range []string{"8080:80", "127.0.0.1:8080:80", "0.0.0.0:53:53/udp", "[::1]:8080:80/tcp", "[::]:443:443", "[2001:db8::1]:9000:9000/sctp", "1:65535"} {
		t.Run(mapping, func(t *testing.T) {
			if err := validatePortMapping(mapping); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, mapping := range []string{"", "80", ":80", "8080:", ":8080:80", "0:80", "8080:0", "65536:80", "80:65536", "-1:80", "+1:80", "8080-8082:80-82", "localhost:8080:80", "999.0.0.1:8080:80", "::1:8080:80", "[::1:8080:80", "8080:80/", "8080:80/http", "8080:80/tcp/udp", " 8080:80", "8080:80\n", "8080:80;touch /tmp/unwanted", "$(id):80", "8080:80'", "--network=host"} {
		t.Run("reject_"+mapping, func(t *testing.T) {
			if err := validatePortMapping(mapping); err == nil {
				t.Fatal("accepted invalid mapping")
			}
		})
	}
}

func TestPortsConfig(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(original)
	defer viper.Reset()
	for _, tc := range []struct {
		name, setting string
		want          []string
		invalid       bool
	}{
		{name: "omitted"},
		{name: "empty", setting: "ports: []\n", want: []string{}},
		{name: "local", setting: "ports:\n  - '127.0.0.1:8080:80'\n  - '[::1]:8080:80/tcp'\n", want: []string{"127.0.0.1:8080:80", "[::1]:8080:80/tcp"}},
		{name: "invalid", setting: "ports:\n  - '0:80'\n", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "lord.yml"), []byte("name: test\nserver: localhost\nweb: false\n"+tc.setting), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			viper.Reset()
			config, err := loadConfig("")
			if tc.invalid {
				if err == nil || !strings.Contains(err.Error(), "invalid ports entry") {
					t.Fatalf("expected ports error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(config.Ports) != len(tc.want) || (len(tc.want) > 0 && !reflect.DeepEqual(config.Ports, tc.want)) {
				t.Fatalf("ports = %#v, want %#v", config.Ports, tc.want)
			}
		})
	}
}

func TestContainerPortsCommand(t *testing.T) {
	for _, web := range []bool{false, true} {
		t.Run(fmt.Sprintf("web_%t", web), func(t *testing.T) {
			config := &Config{GPUs: "all", WebAdvancedConfig: WebAdvancedConfig{MaxRequestBodyBytes: -1, MaxResponseBodyBytes: -1, MemRequestBodyBytes: -1}}
			remote := &remote{config: config}
			baseline := remote.containerRunCommand("test", "image:tag", []string{"/srv/radio:/radio"}, "test.env", web, "radio.example.com")
			if strings.Contains(baseline, "--publish") {
				t.Fatal("unexpected published ports")
			}
			config.Ports = []string{"127.0.0.1:8080:80", "[::1]:8081:81/udp"}
			actual := remote.containerRunCommand("test", "image:tag", []string{"/srv/radio:/radio"}, "test.env", web, "radio.example.com")
			expected := " --publish '127.0.0.1:8080:80' --publish '[::1]:8081:81/udp'"
			if strings.Count(actual, " --publish ") != 2 || strings.Replace(actual, expected, "", 1) != baseline {
				t.Fatalf("unexpected command: %s", actual)
			}
			if !web && strings.Contains(actual, "traefik") {
				t.Fatal("non-web publication enabled Traefik")
			}
		})
	}
}

func TestPortShellQuoting(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("POSIX shell unavailable")
	}
	for _, mapping := range []string{"[::1]:8080:80", "8080:80'; echo unwanted; #", "$(printf unwanted)", "8080:80\nnext"} {
		out, err := exec.Command("sh", "-c", "printf '%s' "+shellQuotePort(mapping)).CombinedOutput()
		if err != nil || string(out) != mapping {
			t.Fatalf("mapping %q: got %q, error %v", mapping, out, err)
		}
	}
}
