package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	clientControlAddr string
	clientDataAddr    string
	pluginAddr        string
	help              bool
)

func init() {
	flag.BoolVar(&help, "h", false, "show help")
	flag.StringVar(&clientControlAddr, "cc", "", "client control address")
	flag.StringVar(&clientDataAddr, "cd", "", "client data address")
	flag.StringVar(&pluginAddr, "p", "", "plugin address")
	flag.BoolVar((*bool)(&Debug), "d", false, "debug log")
}

func main() {
	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(1)
	}

	if clientControlAddr == "" {
		fmt.Println("client control address is nil")
		os.Exit(1)
	}
	if clientDataAddr == "" {
		fmt.Println("client data address is nil")
		os.Exit(1)
	}
	if pluginAddr == "" {
		fmt.Println("plugin address is nil")
		os.Exit(1)
	}
	Debug.SetPrefix("[" + pluginAddr + "]")

	s, err := NewServer(pluginAddr, clientControlAddr, clientDataAddr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = s.Loop()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
