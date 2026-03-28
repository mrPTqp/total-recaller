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
		MaxFileSize  int64  `mapstructure:"max_file_size"`
	} `mapstructure:"bot"`
	Database struct {
		Retry struct {
			MaxAttempts int           `mapstructure:"max_attempts"`
			Backoff     time.Duration `mapstructure:"backoff"`
		} `mapstructure:"retry"`
		Pool struct {
			MaxOpenConns int           `mapstructure:"max_open_conns"`
			MaxIdleConns int           `mapstructure:"max_idle_conns"`
			MaxLifetime  time.Duration `mapstructure:"max_lifetime"`
			MaxIdleTime  time.Duration `mapstructure:"max_idle_time"`
		} `mapstructure:"pool"`
	} `mapstructure:"database"`
	Transcriber struct {
		TokenManager struct {
			Scope           string        `mapstructure:"scope"`
			URL             string        `mapstructure:"url"`
			RefreshInterval time.Duration `mapstructure:"refresh_interval"`
		} `mapstructure:"token_manager"`
		SaluteURL        string `mapstructure:"salute_url"`
		TaskBufferSize   int    `mapstructure:"task_buffer_size"`
		ResultBufferSize int    `mapstructure:"result_buffer_size"`
	} `mapstructure:"transcriber"`
	LLM struct {
		TokenManager struct {
			Scope           string        `mapstructure:"scope"`
			URL             string        `mapstructure:"url"`
			RefreshInterval time.Duration `mapstructure:"refresh_interval"`
		} `mapstructure:"token_manager"`
		Model            string `mapstructure:"model"`
		TaskBufferSize   int    `mapstructure:"task_buffer_size"`
		ResultBufferSize int    `mapstructure:"result_buffer_size"`
	} `mapstructure:"llm"`
	// Sensitive data loaded only from environment variables
	BotToken                           string `mapstructure:"-"`
	DatabaseDSN                        string `mapstructure:"-"`
	TranscriberTokenManagerCredentials string `mapstructure:"-"`
	LLMTokenManagerCredentials         string `mapstructure:"-"`
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
	cfg.TranscriberTokenManagerCredentials = v.GetString("TRANSCRIBER_TOKEN_MANAGER_CREDENTIALS")
	cfg.LLMTokenManagerCredentials = v.GetString("LLM_TOKEN_MANAGER_CREDENTIALS")

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
	v.SetDefault("bot.max_file_size", 20971520) // 20MB in bytes
	v.SetDefault("database.retry.max_attempts", 3)
	v.SetDefault("database.retry.backoff", "1s")
	v.SetDefault("database.pool.max_open_conns", 25)
	v.SetDefault("database.pool.max_idle_conns", 5)
	v.SetDefault("database.pool.max_lifetime", "5m")
	v.SetDefault("database.pool.max_idle_time", "1m")
	v.SetDefault("transcriber.token_manager.scope", "SALUTE_SPEECH_PERS")
	v.SetDefault("transcriber.token_manager.url", "https://ngw.devices.sberbank.ru:9443/api/v2/oauth")
	v.SetDefault("transcriber.token_manager.refresh_interval", "29m")
	v.SetDefault("transcriber.salute_url", "https://smartspeech.sber.ru/rest/v1")
	v.SetDefault("transcriber.task_buffer_size", 100)
	v.SetDefault("transcriber.result_buffer_size", 100)
	v.SetDefault("llm.token_manager.scope", "GIGACHAT_API_PERS")
	v.SetDefault("llm.token_manager.url", "https://ngw.devices.sberbank.ru:9443/api/v2/oauth")
	v.SetDefault("llm.token_manager.refresh_interval", "29m")
	v.SetDefault("llm.model", "GigaChat-2")
	v.SetDefault("llm.task_buffer_size", 100)
	v.SetDefault("llm.result_buffer_size", 100)
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
	if cfg.TranscriberTokenManagerCredentials == "" {
		return fmt.Errorf("transcriber token manager credentials are required")
	}
	return nil
}

// String implements custom string representation for Config
func (c *Config) String() string {
	return fmt.Sprintf("Config{Bot: {Username: %s, PoolSize: %d, MaxQueueSize: %d, MaxFileSize: %d}, Database: {Retry: {MaxAttempts: %d, Backoff: %s}, Pool: {MaxOpenConns: %d, MaxIdleConns: %d, MaxLifetime: %s, MaxIdleTime: %s}}, Transcriber: {TokenManager: {Scope: %s, URL: %s, RefreshInterval: %s}, SaluteURL: %s, TaskBufferSize: %d, ResultBufferSize: %d}, LLM: {TokenManager: {Scope: %s, URL: %s, RefreshInterval: %s}, Model: %s, TaskBufferSize: %d, ResultBufferSize: %d}}}",
		c.Bot.Username, c.Bot.PoolSize, c.Bot.MaxQueueSize, c.Bot.MaxFileSize,
		c.Database.Retry.MaxAttempts, c.Database.Retry.Backoff,
		c.Database.Pool.MaxOpenConns, c.Database.Pool.MaxIdleConns,
		c.Database.Pool.MaxLifetime, c.Database.Pool.MaxIdleTime,
		c.Transcriber.TokenManager.Scope, c.Transcriber.TokenManager.URL, c.Transcriber.TokenManager.RefreshInterval,
		c.Transcriber.SaluteURL, c.Transcriber.TaskBufferSize, c.Transcriber.ResultBufferSize,
		c.LLM.TokenManager.Scope, c.LLM.TokenManager.URL, c.LLM.TokenManager.RefreshInterval,
		c.LLM.Model, c.LLM.TaskBufferSize, c.LLM.ResultBufferSize)
}
