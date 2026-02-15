package hasher

import (
	"github.com/code19m/errx"
	"golang.org/x/crypto/bcrypt"
)

type opts struct {
	cost int
}

type Option func(*opts)

func WithCost(cost int) Option {
	return func(o *opts) {
		o.cost = cost
	}
}

func Hash(password string, options ...Option) (string, error) {
	o := opts{cost: bcrypt.DefaultCost}
	for _, opt := range options {
		opt(&o)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), o.cost)
	if err != nil {
		return "", errx.Wrap(err)
	}
	return string(hash), nil
}

func Compare(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
