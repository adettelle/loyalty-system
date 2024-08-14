package config

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Address              string `env:"RUN_ADDRESS" flag:"a" default:"localhost:8080"`
	DBUri                string `env:"DATABASE_URI" flag:"d" default:""`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS" flag:"r" default:"localhost:8081"`
}

func New() (*Config, error) {
	var cfg Config

	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	flag.StringVar(&cfg.Address, "a", cfg.Address, "Net address localhost:port")
	flag.StringVar(&cfg.DBUri, "d", cfg.DBUri, "db connection params")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", cfg.AccrualSystemAddress,
		"Accrual system address localhost:port")

	flag.Parse()

	ensureAddrFLagIsCorrect(cfg.Address)

	return &cfg, nil
}

func ensureAddrFLagIsCorrect(addr string) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Fatal(err)
	}

	_, err = strconv.Atoi(port)
	if err != nil {
		log.Fatal(fmt.Errorf("invalid port: '%s'", port))
	}
}
