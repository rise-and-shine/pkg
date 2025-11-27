package main

import (
	"context"
	"time"

	"github.com/rise-and-shine/pkg/logger"
	"github.com/rise-and-shine/pkg/mask"
	"github.com/rise-and-shine/pkg/pg"
	"github.com/rise-and-shine/pkg/repogen"
	"github.com/uptrace/bun"
)

func main() {
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
		Password: "password",
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

	// logger.Info("qq", "listjon", map[string]any{
	// 	"qq": qq,
	// })
}

type Request struct {
	Username string
	Password string `mask:"true"`
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
	ID           int32
	Email        string
	Username     string
	PasswordHash string
	Role         string
	ImagePath    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
