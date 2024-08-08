package config

import (
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type EnvType string

const (
	EnvDev  EnvType = "dev"
	EnvProd EnvType = "prod"
)

type Server struct {
	Env        EnvType `yaml:"env" env-default:"prod"`
	PublicPort int     `yaml:"public_port" env-default:"8000"`
	ClientPort int     `yaml:"client_port" env-default:"9000"`
}

type Client struct {
	Env        EnvType       `yaml:"env" env-default:"prod"`
	Port       int           `yaml:"port" env-default:"8080"`
	ServerAddr string        `yaml:"server_addr" env-default:"127.0.0.1:8080"`
	Timeout    time.Duration `yaml:"timeout" env-default:"60s"`
}

func (e EnvType) LogLevel() slog.Level {
	switch e {
	case EnvProd:
		return slog.LevelInfo
	case EnvDev:
		return slog.LevelDebug
	}

	panic("invalid env")
}

func getPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}

func MustLoad[T any]() T {
	configPath := getPath()

	if configPath == "" {
		panic("please set config path via --config flag or CONFIG_PATH env variable")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config path does not exist: " + configPath)
	}

	var cfg T

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("failed to read config: " + err.Error())
	}

	return cfg
}
