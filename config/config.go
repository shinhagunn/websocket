package config

import (
	"github.com/caarlos0/env"
	"github.com/cockroachdb/errors"
	"github.com/zsmartex/pkg/v2/config"
)

type Rango struct {
	RbacSystem []string `env:"RANGO_RBAC_SYSTEM" envDefault:"admin,superadmin,operator"`
	RbacAdmin  []string `env:"RANGO_RBAC_ADMIN" envDefault:"admin,superadmin"`
}

type Config struct {
	HTTP            config.HTTP
	Kafka           config.Kafka
	Rango           Rango
	ApplicationName string `env:"APP_NAME" envDefault:"Rango"`
	JWTPublicKey    string `env:"JWT_PUBLIC_KEY"`
}

func NewConfig() (*Config, error) {
	conf := new(Config)

	if err := env.Parse(conf); err != nil {
		return nil, errors.Newf("parse config: %v", err)
	}

	return conf, nil
}
