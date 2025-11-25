package main

import "github.com/rise-and-shine/pkg/logger"

func main() {
	cfg := logger.Config{
		Level:    "debug",
		Encoding: "pretty",
	}
	_ = cfg
	// logger.SetGlobal(cfg)

	// logger, err := logger.New(cfg)
	// if err != nil {
	// 	panic(err)
	// }

	logger.Debug("qweqwer")
}
