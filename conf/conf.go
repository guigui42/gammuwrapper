package conf

import (
	"errors"
	"os"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/rs/zerolog/log"
)

var K = koanf.New(".")

type Config struct {
	GammuConf       string `koanf:"GAMMUCONF"`
	Port            int    `koanf:"SERVERPORT"`
	SMSQueueMaxSize int    `koanf:"SMSQUEUEMAXSIZE"`
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
		"GAMMUCONF":       "/etc/gammu-smsdrc",
		"SERVERPORT":      8083,
		"SMSQUEUEMAXSIZE": 10,
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

	log.Trace().Msgf("CONF is %%+v: %+v\n", Conf)
	return nil
}
