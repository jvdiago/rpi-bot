package main

import (
	"context"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Command struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type Config struct {
	Commands map[string]Command `yaml:"commands"`
	Signal   SignalConfig       `yaml:"signal"`
	Telegram TelegramConfig     `yaml:"telegram"`
	Provider string             `yaml:"provider"`
	Httpd    HttpdConfig        `yaml:"httpd"`
}

type TelegramConfig struct {
	Debug    bool   `yaml:"debug"`
	ApiToken string `yaml:"apiToken"`
}
type SignalConfig struct {
	Sources []string `yaml:"sources"`
	Socket  string   `yaml:"socket"`
}
type HttpdConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Addr      string `yaml:"addr"`
	AuthToken string `yaml:"authToken"`
}

// NewConfig returns a new decoded Config struct
func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("error closing file: %v", err)
		}
	}()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// ValidateConfigPath just makes sure, that the path provided is a file,
// that can be read
func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}

// ParseFlags will create and parse the CLI flags
// and return the path to be used elsewhere
func ParseFlags() (string, error) {
	// String that contains the configured configuration path
	var configPath string

	// Set up a CLI flag called "-config" to allow users
	// to supply the configuration file
	flag.StringVar(&configPath, "config", "./config.yaml", "path to config file")

	// Actually parse the flags
	flag.Parse()

	// Validate the path first
	if err := ValidateConfigPath(configPath); err != nil {
		return "", err
	}

	// Return the configuration path
	return configPath, nil
}

// Return a secret found in an ENV var or in config.yaml. ENV var has precedence
func GetSecret(envVar string, cfgSetting string) (string, bool) {
	var secret string

	secret, exists := os.LookupEnv(envVar)
	if exists {
		return secret, true
	}

	if cfgSetting != "" {
		return cfgSetting, true
	}

	return "", false

}

func main() {
	cfgPath, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := NewConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigs
		log.Println("Signal received. Terminating")
		cancel()
	}()

	sr, err := MessagingFactory(cfg)

	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	exec := &executor{}

	if cfg.Httpd.Enabled {
		wg.Add(1)
		go HttpServer(ctx, cfg, exec, &wg)
	}
	if sr != nil {
		wg.Add(1)
		go MessagingPoller(ctx, sr, exec, cfg.Commands, &wg)
	}
	wg.Wait()

}
