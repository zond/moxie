package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zond/moxie/consumer"
	"github.com/zond/moxie/controller"
	"github.com/zond/moxie/logger"
	"github.com/zond/moxie/proxy"
)

const (
	modeConsume = "consume"
	modeControl = "control"
	modeProxy   = "proxy"
	modeLog     = "log"
)

var modes = []string{
	modeConsume,
	modeControl,
	modeProxy,
	modeLog,
}

func main() {
	defaultDir := filepath.Join(os.Getenv("HOME"), ".moxie")
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
		if err := proxy.Connect(*remotehost, nil); err != nil {
			panic(err)
		}
		if err := proxy.Publish(struct{}{}, nil); err != nil {
			panic(err)
		}
	case modeConsume:
		consumer := consumer.New()
		if err := consumer.Publish(struct{}{}, nil); err != nil {
			panic(err)
		}
	case modeControl:
		controller := controller.New().Dir(*dir)
		if err := controller.Control(struct{}{}, nil); err != nil {
			panic(err)
		}
	case modeLog:
		logger := logger.New()
		if err := logger.Publish(struct{}{}, nil); err != nil {
			return
		}
	}

}
