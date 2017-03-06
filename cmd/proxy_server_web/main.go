package main

import (
	"flag"
	"log"
	"os"

	"github.com/tw4452852/proxy_server"
)

func main() {
	help := flag.Bool("h", false, "show help")
	flag.BoolVar((*bool)(&proxy_server.Debug), "d", false, "debug log")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(1)
	}

	w := proxy_server.NewWeb()
	defer w.Exit()

	p, err := w.Address()
	if err != nil {
		log.Fatalln(err)
	}

	c, d, err := w.GetVmAddress()
	if err != nil {
		log.Fatalln(err)
	}

	s, err := proxy_server.NewServer(p, c, d)
	if err != nil {
		log.Fatalln(err)
	}

	err = s.Loop()
	if err != nil {
		log.Fatalln(err)
	}
}
