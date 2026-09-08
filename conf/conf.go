package conf

import (
	"errors"
	"fmt"
	"os"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/rs/zerolog/log"
)

var K = koanf.New(".")

const (
	defaultGammuConf                     = "/etc/gammu-smsdrc"
	defaultPort                          = 8083
	defaultSMSQueueMaxSize               = 10
	defaultGammuSendTimeoutSeconds       = 45
	defaultGammuDiagnosticTimeoutSeconds = 10
)

type Config struct {
	GammuConf                     string `koanf:"GAMMUCONF"`
	Port                          int    `koanf:"SERVERPORT"`
	SMSQueueMaxSize               int    `koanf:"SMSQUEUEMAXSIZE"`
	GammuSendTimeoutSeconds       int    `koanf:"GAMMUSENDTIMEOUTSECONDS"`
	GammuDiagnosticTimeoutSeconds int    `koanf:"GAMMUDIAGNOSTICTIMEOUTSECONDS"`
}

var Conf Config

func CheckFileExists(filePath string) bool {
	_, error := os.Stat(filePath)

	return !errors.Is(error, os.ErrNotExist)
}

func LoadConf() error {
	// CONFIGURATION
	// Load environment variables

	// Loading Default values
	err := K.Load(confmap.Provider(map[string]interface{}{
		"GAMMUCONF":                     defaultGammuConf,
		"SERVERPORT":                    defaultPort,
		"SMSQUEUEMAXSIZE":               defaultSMSQueueMaxSize,
		"GAMMUSENDTIMEOUTSECONDS":       defaultGammuSendTimeoutSeconds,
		"GAMMUDIAGNOSTICTIMEOUTSECONDS": defaultGammuDiagnosticTimeoutSeconds,
	}, "."), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("error loading default config")
	}

	// Load .env file
	if CheckFileExists(".env") {
		if err := K.Load(file.Provider(".env"), dotenv.Parser()); err != nil {
			log.Fatal().Err(err).Msg("error loading config .env file")
		}
	} else {
		// load environment variables
		err := K.Load(env.Provider("", ".", nil), nil)
		if err != nil {
			log.Fatal().Err(err).Msg("error loading env variable config")
		}
	}

	// Quick unmarshal.
	err = K.Unmarshal("", &Conf)
	if err != nil {
		log.Fatal().Err(err).Msg("error Unmarshal config")
	}

	return Conf.Validate()
}

func (c Config) Validate() error {
	if c.GammuSendTimeoutSeconds <= 0 {
		return fmt.Errorf("GAMMUSENDTIMEOUTSECONDS must be positive")
	}
	if c.GammuDiagnosticTimeoutSeconds <= 0 {
		return fmt.Errorf("GAMMUDIAGNOSTICTIMEOUTSECONDS must be positive")
	}
	return nil
}
