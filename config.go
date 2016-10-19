package main

import (
	"io"

	"github.com/BurntSushi/toml"
)

type config struct {
	Web webConfig
}

type webConfig struct {
	Url string
}

func getConfig(r io.Reader) (*config, error) {
	c := &config{}
	_, err := toml.DecodeReader(r, c)
	if err != nil {
		return nil, err
	}
	return c, nil
}
