package main

import (
	"github.com/rise-and-shine/pkg/logger"
	"github.com/rise-and-shine/pkg/mask"
)

func main() {
	cfg := logger.Config{
		Level:    "debug",
		Encoding: "json",
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
