package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ayinke-llc/sdump/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version describes the version of the current build.
	Version = "dev"

	// Commit describes the commit of the current build.
	Commit = "none"

	// Date describes the date of the current build.
	Date = time.Now().UTC()
)

const (
	defaultConfigFilePath = "config"
	envPrefix             = "SDUMP"
)

func main() {
	if err := Execute(); err != nil {
		log.Fatal(err)
	}
}

func initializeConfig(cfg *config.Config) error {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	setDefaults()

	viper.AddConfigPath(filepath.Join(homePath, ".config", defaultConfigFilePath))
	viper.AddConfigPath(".")

	viper.SetConfigName(defaultConfigFilePath)
	viper.SetConfigType("yml")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	viper.SetEnvPrefix(envPrefix)

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return viper.Unmarshal(cfg)
}

func Execute() error {
	cfg := &config.Config{}

	rootCmd := &cobra.Command{
		Use:   "sdump",
		Short: "sdump runs a SSH server that helps you view and inspect incoming http requests",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initializeConfig(cfg)
		},
	}

	rootCmd.SetVersionTemplate(
		fmt.Sprintf("Version: %v\nCommit: %v\nDate: %v\n", Version, Commit, Date))

	rootCmd.Flags().StringP("config", "c", defaultConfigFilePath, "Config file. This is in YAML")

	createHTTPCommand(rootCmd, cfg)
	createSSHCommand(rootCmd, cfg)
	createDeleteCommand(rootCmd, cfg)

	return rootCmd.Execute()
}

func setDefaults() {
	viper.SetDefault("tui.color_scheme", "monokai")
	viper.SetDefault("log_level", "debug")
	viper.SetDefault("ssh.port", 2222)
	viper.SetDefault("ssh.host", "localhost")
	viper.SetDefault("ssh.identities", []string{".ssh/id_rsa"})
	viper.SetDefault("http.database.log_queries", false)
	viper.SetDefault("http.database.dsn", "postgres://sdump:sdump@localhost:3432/sdump?sslmode=disable")
	viper.SetDefault("http.port", 4200)
	viper.SetDefault("http.domain", "sdump.app")
	viper.SetDefault("http.max_request_body_size", 1024)
	viper.SetDefault("http.prometheus.is_enabled", false)
	viper.SetDefault("http.prometheus.username", "")
	viper.SetDefault("http.prometheus.password", "")
	viper.SetDefault("http.otel.is_enabled", false)
	viper.SetDefault("http.otel.use_tls", true)
	viper.SetDefault("http.otel.service_name", "SDUMP")
	viper.SetDefault("http.rate_limit.requests_per_minute", 60)
	viper.SetDefault("http.otel.endpoint", "localhost:9500")
	viper.SetDefault("cron.soft_deletes", false)
	viper.SetDefault("cron.ttl", "48h")
}
