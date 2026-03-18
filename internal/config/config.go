package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv      string
	AppPort     string
	DatabaseURL string
	OpenAIKey   string
	NewsAPIKey  string
}

var C Config

func Init() {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("no .env file found, using environment variables")
		}
	}

	C = Config{
		AppEnv:      viper.GetString("APP_ENV"),
		AppPort:     viper.GetString("APP_PORT"),
		DatabaseURL: viper.GetString("DATABASE_URL"),
		OpenAIKey:   viper.GetString("OPENAI_API_KEY"),
		NewsAPIKey:  viper.GetString("NEWS_API_KEY"),
	}

	if C.AppPort == "" {
		C.AppPort = "8080"
	}
	if C.AppEnv == "" {
		C.AppEnv = "local"
	}
}
