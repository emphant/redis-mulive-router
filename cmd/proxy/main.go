package main

import (
	"github.com/emphant/redis-mulive-router/pkg/proxy"
	"github.com/emphant/redis-mulive-router/pkg/utils/log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	config := proxy.NewDefaultConfig()
	s, err := proxy.New(config)

	start(s)

	if err != nil {
		log.PanicErrorf(err, "create proxy with config file failed\n%s", config)
	}
	defer s.Close()

	go func() {
		defer s.Close()
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)

		sig := <-c
		log.Warnf("[%p] proxy receive signal = '%v'", s, sig)
	}()

	for !s.IsClosed() && !s.IsOnline() {//等待启动
		log.Warnf("[%p] proxy waiting online ...", s)
		time.Sleep(time.Second)
	}

	log.Warnf("[%p] proxy is working ...", s)

	for !s.IsClosed() {
		time.Sleep(time.Second)
	}

	log.Warnf("[%p] proxy is exiting ...", s)
}

func start(s *proxy.Proxy)  {
	if err := s.Start(); err != nil {
		log.PanicErrorf(err, "start proxy failed")
	}
}