package config

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// application configuration
type Config struct {
	Bot struct {
		Username     string `mapstructure:"username"`
		PoolSize     int    `mapstructure:"pool_size"`   
		MaxQueueSize int    `mapstructure:"max_queue_size"` 
	} `mapstructure:"bot"`
	// Sensitive data loaded only from environment variables
	BotToken string `mapstructure:"-"`
}

func LoadConfig() (*Config, error) {
	v := viper.New()
	setConfigPaths(v)

	setDefaults(v)
	loadConfigFile(v)
	loadEnvironment(v)
	loadFlags(v)
	
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}
	
	cfg.BotToken = v.GetString("BOT_TOKEN")
	
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	
	return &cfg, nil
}

func loadFlags(v *viper.Viper) {
	pflag.String("bot.username", "", "Bot username (overrides config)")
	pflag.Parse()
	v.BindPFlags(pflag.CommandLine)
}

func setConfigPaths(v *viper.Viper) {
	v.SetConfigName("config")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("$HOME/.total-recaller")
	v.AddConfigPath("/etc/total-recaller/")
}

func loadConfigFile(v *viper.Viper) {
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("Config file not found, using defaults and environment variables")
		} else {
			panic(fmt.Errorf("error reading config file: %w", err))
		}
	} else {
		fmt.Printf("Using config file: %s\n", v.ConfigFileUsed())
	}
}

func loadEnvironment(v *viper.Viper) {
	v.SetEnvPrefix("TOTAL_RECALLER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
}

// sets default values for configuration
func setDefaults(v *viper.Viper) {
	v.SetDefault("bot.username", "")
	v.SetDefault("bot.pool_size", 10)      
	v.SetDefault("bot.max_queue_size", 100) 
}

// validates configuration
func validateConfig(cfg *Config) error {
	if cfg.Bot.Username == "" {
		return fmt.Errorf("bot username is required")
	}
	if cfg.BotToken == "" {
		return fmt.Errorf("bot token is required and must be set via TOTAL_RECALLER_BOT_TOKEN environment variable")
	}
	return nil
}
