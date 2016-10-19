package main

import (
	"flag"
	"log"
	"os"
)

var (
	configPath string
	debug      bool
	// TODO: remove this when release
	clientAddr string
)

func init() {
	flag.StringVar(&configPath, "c", "config.toml", "config file")
	flag.StringVar(&clientAddr, "t", "", "client address")
	flag.BoolVar(&debug, "d", false, "debug log")
}

func main() {
	flag.Parse()

	// TODO: remove this when release
	if clientAddr == "" {
		clientAddr, err := getClientAddr(configPath)
		if err != nil {
			log.Fatal(err)
		}

		log.Fatal(clientLoop(clientAddr))
	}
}

func getClientAddr(cp string) (string, error) {
	// open the config file
	f, err := os.Open(cp)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// get the config
	c, err := getConfig(f)
	if err != nil {
		return "", err
	}

	// query the web
	return queryClientAddr(c.Web.Url)
}
