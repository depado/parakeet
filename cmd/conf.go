package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/spf13/viper"
)

// LogConf holds the configuration for the application logger
type LogConf struct {
	Level  string `mapstructure:"level"`
	Type   string `mapstructure:"type"`
	Caller bool   `mapstructure:"caller"`
}

// Conf holds the various configuration options for our application
type Conf struct {
	Log       LogConf `mapstructure:"log"`
	UserID    string  `mapstructure:"user_id"`
	AuthToken string  `mapstructure:"auth_token"` // Authentication Token
	URL       string
    Shuffle   bool    `mapstructure:"shuffle"`
}

// NewLogger will return a new logger
func NewLogger(c *Conf) zerolog.Logger {
	// Level parsing
	warns := []string{}
	lvl, err := zerolog.ParseLevel(c.Log.Level)
	if err != nil {
		warns = append(warns, fmt.Sprintf("unrecognized log level '%s', fallback to 'info'", c.Log.Level))
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(lvl)
	}

	// Type parsing
	switch c.Log.Type {
	case "console":
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	case "json":
		break
	default:
		warns = append(warns, fmt.Sprintf("unrecognized log type '%s', fallback to 'json'", c.Log.Type))
	}

	// Caller
	if c.Log.Caller {
		log.Logger = log.With().Caller().Logger()
	}

	// Log messages with the newly created logger
	for _, w := range warns {
		log.Warn().Msg(w)
	}

	return log.Logger
}

// NewConf will parse and return the configuration
func NewConf() (*Conf, error) {
	// Environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("parakeet")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Configuration file
	if viper.GetString("conf") != "" {
		viper.SetConfigFile(viper.GetString("conf"))
	} else {
		viper.SetConfigName("conf")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/config/")
	}

	viper.ReadInConfig() // nolint: errcheck
	conf := &Conf{}
	if err := viper.Unmarshal(conf); err != nil {
		return conf, fmt.Errorf("unable to unmarshal conf: %w", err)
	}

	return conf, nil
}

