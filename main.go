package main

import (
	"context"
	"time"

	"github.com/rise-and-shine/pkg/cfgloader"
	"github.com/rise-and-shine/pkg/logger"
	"github.com/rise-and-shine/pkg/mask"
	"github.com/rise-and-shine/pkg/pg"
	"github.com/rise-and-shine/pkg/repogen"
	"github.com/uptrace/bun"
)

type Config struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password" mask:"true"`

	SecretObject *struct {
		Objer1 string
		Objer2 bool
	} `yaml:"secret_object" mask:"true"`
}

func main() {
	c := cfgloader.MustLoad[Config]()

	logger.With("config", mask.StructToOrdMap(c)).Info("loaded config:")

	cfg := logger.Config{
		Level:    "debug",
		Encoding: "pretty",
	}
	_ = cfg
	logger.SetGlobal(cfg)

	// logger := logger.Named("mainjon.main")

	logger, err := logger.New(cfg)
	if err != nil {
		panic(err)
	}

	r := Request{
		Username: "username",
		Password: "",
	}

	resp := Response{
		Username: "username",
		Age:      10,
		InnerObject: &struct {
			Name string
		}{
			Name: "name",
		},
		InnarMap: map[string]string{
			"key": "value",
		},
	}

	respNil := Response{
		Username:    "username",
		Age:         10,
		InnerObject: nil,
		InnarMap:    nil,
	}

	logger.With("request_body", mask.StructToOrdMap(r)).Info("requestjon")
	logger.With("request_body_no_mask", r).Info("requestjon")
	logger.With("response_body", mask.StructToOrdMap(resp)).Info("responsejon")
	logger.With("response_bodynil", mask.StructToOrdMap(respNil)).Info("responsejonnil")

	logger.Debug("qweqwer")

	bd, err := pg.NewBunDB(pg.Config{
		Debug:               true,
		Host:                "localhost",
		Port:                5432,
		User:                "postgres",
		Password:            "postgres",
		Database:            "chatx",
		SSLMode:             "disable",
		SearchPath:          "public",
		ConnectTimeout:      10,
		PoolMaxConns:        4,
		PoolMinConns:        4,
		PoolMaxConnLifetime: 1,
		PoolMaxConnIdleTime: 1,
	})
	if err != nil {
		logger.Fatalx(err)
	}

	rep := repogen.NewPgRepo[User](
		bd,
		"user",
		"USER_NOT_FOUND",
		map[string]string{},
		func(q *bun.SelectQuery, f struct{}) *bun.SelectQuery {
			return q
		},
	)

	qq, err := rep.List(context.Background(), struct{}{})
	if err != nil {
		logger.Fatalx(err)
	}

	logger.With("qq", qq).Info("qq")
}

type Request struct {
	Username string `json:"username"`
	Password string `json:"password" mask:"true"`
}

type Response struct {
	Username string
	Age      int

	InnerObject *struct {
		Name string
	} `mask:"true"`

	InnarMap map[string]string `mask:"true"`
}

type User struct {
	ID           int32  `json:"id"    bun:"id,pk,autoincrement"`
	Email        string `json:"email" bun:"email,notnull"`
	Username     string
	PasswordHash string
	Role         string
	ImagePath    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
