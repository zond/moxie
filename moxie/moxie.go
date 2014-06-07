package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zond/moxie/consumer"
	"github.com/zond/moxie/controller"
	"github.com/zond/moxie/proxy"
)

const (
	modeConsume = "consume"
	modeControl = "control"
	modeProxy   = "proxy"
)

var modes = []string{
	modeConsume,
	modeControl,
	modeProxy,
}

func main() {
	defaultDir := filepath.Join(os.Getenv("HOME"), ".moxie")
	localhost := flag.String("localhost", "localhost:6677", "The local host for the proxy.")
	remotehost := flag.String("remotehost", "", fmt.Sprintf("Where to connect to. Required for %v mode.", modeProxy))
	dir := flag.String("dir", defaultDir, "Where to store persistent data like history and logs.")
	mode := flag.String("mode", modeProxy, fmt.Sprintf("The run mode, one of %v.", modes))

	flag.Parse()

	switch *mode {
	case modeProxy:
		if *remotehost == "" {
			flag.Usage()
			return
		}
		proxy := proxy.New()
		if err := proxy.Connect(*remotehost, &struct{}{}); err != nil {
			panic(err)
		}
		if err := proxy.Listen(*localhost, &struct{}{}); err != nil {
			panic(err)
		}
	case modeConsume:
		parts := strings.Split(*localhost, ":")
		consumer := consumer.New()
		if err := consumer.Listen(parts[0], &struct{}{}); err != nil {
			panic(err)
		}
		if err := consumer.Connect(*localhost, &struct{}{}); err != nil {
			panic(err)
		}
	case modeControl:
		controller := controller.New().Dir(*dir)
		if err := controller.Connect(*localhost, &struct{}{}); err != nil {
			panic(err)
		}
		if err := controller.Control(struct{}{}, &struct{}{}); err != nil {
			panic(err)
		}
	}

}
