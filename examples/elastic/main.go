package main

import (
	goservice "github.com/200lab/go-sdk"
	"github.com/200lab/go-sdk/plugin/storage/sdkes"
	"github.com/sirupsen/logrus"
)

func main() {
	service := goservice.New(
		goservice.WithName("demo"),
		goservice.WithVersion("1.0.0"),
	)
	_ = service.Init()
	newES := sdkes.NewES("test", "example")
	newES.InitFlags()
	err := newES.Run()
	if err != nil {
		logrus.Error("err: ", err)
	}
	_ = service.Start()
}
