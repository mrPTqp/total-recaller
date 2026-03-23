package config

import (
	"fmt"
	"strings"
	"time"

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
	Database struct {
		Retry struct {
			MaxAttempts int           `mapstructure:"max_attempts"`
			Backoff     time.Duration `mapstructure:"backoff"`
		} `mapstructure:"retry"`
		Pool struct {
			MaxOpenConns    int           `mapstructure:"max_open_conns"`
			MaxIdleConns    int           `mapstructure:"max_idle_conns"`
			MaxLifetime     time.Duration `mapstructure:"max_lifetime"`
			MaxIdleTime     time.Duration `mapstructure:"max_idle_time"`
		} `mapstructure:"pool"`
	} `mapstructure:"database"`
	// Sensitive data loaded only from environment variables
	BotToken string `mapstructure:"-"`
	DatabaseDSN string `mapstructure:"-"`
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
	cfg.DatabaseDSN = v.GetString("DATABASE_DSN")
	
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
	v.SetDefault("database.retry.max_attempts", 3)
	v.SetDefault("database.retry.backoff", "1s") // time.Duration format
	v.SetDefault("database.pool.max_open_conns", 25)
	v.SetDefault("database.pool.max_idle_conns", 5)
	v.SetDefault("database.pool.max_lifetime", "5m")
	v.SetDefault("database.pool.max_idle_time", "1m")
}

// validates configuration
func validateConfig(cfg *Config) error {
	if cfg.Bot.Username == "" {
		return fmt.Errorf("bot username is required")
	}
	if cfg.BotToken == "" {
		return fmt.Errorf("bot token is required")
	}
	if cfg.DatabaseDSN == "" {
		return fmt.Errorf("database DSN is required")
	}
	return nil
}
