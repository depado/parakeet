package cmd

import (
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AddFlags will add all the flags provided in this package to the provided
// command and will bind those flags with viper
func AddFlags(c *cobra.Command) {
	c.PersistentFlags().BoolP("debug", "d", false, "enable debug logs")
	c.PersistentFlags().StringP("conf", "c", "", "configuration file to use")
	c.PersistentFlags().StringP("client_id", "i", "", "client id for the soundcloud API")
	c.PersistentFlags().Uint64P("user_id", "u", 0, "user id to fetch data from")
	if err := viper.BindPFlags(c.PersistentFlags()); err != nil {
		logrus.WithError(err).WithField("step", "AddAllFlags").Fatal("Couldn't bind flags")
	}
}

// Initialize will be run when cobra finishes its initialization
func Initialize() {
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
	hasconf := viper.ReadInConfig() == nil

	if viper.GetBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Delays the log for once the logger has been setup
	if hasconf {
		logrus.WithField("file", viper.ConfigFileUsed()).Debug("Found configuration file")
	} else {
		logrus.Debug("No configuration file found")
	}
}
