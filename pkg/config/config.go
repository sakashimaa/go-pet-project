package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
)

type Config struct {
	Env      string   `yaml:"env" env:"ENV" env-default:"local"`
	HTTP     HTTP     `yaml:"http"`
	GRPC     GRPC     `yaml:"grpc"`
	Postgres PG       `yaml:"postgres"`
	Redis    Redis    `yaml:"redis"`
	Services Services `yaml:"services"`
	Limiter  Limiter  `yaml:"limiter"`
}

type HTTP struct {
	Port    string        `yaml:"port" env:"HTTP_PORT" env-default:":3000"`
	Timeout time.Duration `yaml:"timeout" env-default:"4s"`
}

type GRPC struct {
	Port    string        `yaml:"port" env:"GRPC_PORT" env-default:":50051"`
	Timeout time.Duration `yaml:"timeout" env-default:"4s"`
}

type PG struct {
	URL string `yaml:"url" env:"DB_URL"`
}

type Redis struct {
	Addr string `yaml:"addr" env:"REDIS_ADDR" env-default:"localhost:6379"`
}

type Services struct {
	AuthRPC    string `yaml:"auth_rpc" env:"AUTH_RPC_URL"`
	ProductRPC string `yaml:"product_rpc" env:"PRODUCT_RPC_URL"`
}

type Limiter struct {
	RPC   int           `yaml:"rpc" env-default:"10"`
	Burst int           `yaml:"burst" env-default:"20"`
	TTL   time.Duration `yaml:"ttl" env-default:"1m"`
}

func MustLoad() *Config {
	configPath := utils.ParseWithFallback("CONFIG_PATH", "./config/local.yaml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exists: %v\n", err)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("error reading config: %v", err)
	}

	return &cfg
}
