// Copyright (c) 2019, Viet Tran, 200Lab Team.

package goservice

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/baozhenglab/go-sdk/v2/util"
	"github.com/gofiber/fiber/v2"

	"github.com/baozhenglab/go-sdk/v2/httpserver"
	"github.com/baozhenglab/go-sdk/v2/logger"

	"github.com/joho/godotenv"
	"github.com/olekukonko/tablewriter"
)

const (
	DevEnv     = "dev"
	StgEnv     = "stg"
	PrdEnv     = "prd"
	DefaultEnv = DevEnv
)

type service struct {
	name              string
	version           string
	env               string
	opts              []Option
	subServices       []Runnable
	initServices      map[string]PrefixRunnable
	configureServices map[string]PrefixConfigure
	isRegister        bool
	logger            logger.Logger
	hasHttp           bool
	httpServer        HttpServer
	signalChan        chan os.Signal
	cmdLine           *AppFlagSet
	stopFunc          func()
}

func New(opts ...Option) Service {
	sv := &service{
		opts:              opts,
		signalChan:        make(chan os.Signal, 1),
		subServices:       []Runnable{},
		initServices:      map[string]PrefixRunnable{},
		configureServices: map[string]PrefixConfigure{},
		hasHttp:           true,
	}

	for _, opt := range opts {
		opt(sv)
	}
	return sv
}

func (s *service) Name() string {
	return s.name
}

func (s *service) Version() string {
	return s.version
}

func (s *service) Init() error {
	for _, dbSv := range s.initServices {
		if err := dbSv.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (s *service) IsRegistered() bool {
	return s.isRegister
}

func (s *service) SetHTTPServer(has bool) Service {
	s.hasHttp = has
	return s
}

func (s *service) Create() Service {
	// init default logger
	logger.InitServLogger(false)
	s.logger = logger.GetCurrent().GetLogger("service")

	if s.hasHttp {
		//// Http server
		httpServer := httpserver.New(s.name)
		s.httpServer = httpServer

		s.subServices = append(s.subServices, httpServer)
	}

	s.initFlags()

	if s.name == "" {
		if len(os.Args) >= 2 {
			s.name = strings.Join(os.Args[:2], " ")
		}
	}

	loggerRunnable := logger.GetCurrent().(Runnable)
	loggerRunnable.InitFlags()
	_ = loggerRunnable.Configure()

	s.cmdLine = newFlagSet(s.name, flag.CommandLine)
	s.parseFlags()

	return s
}

func (s *service) Start() error {
	signal.Notify(s.signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	c := s.run()
	//s.stopFunc = s.activeRegistry()

	for {
		select {
		case err := <-c:
			if err != nil {
				s.logger.Error(err.Error())
				s.Stop()
				return err
			}

		case sig := <-s.signalChan:
			s.logger.Infoln(sig)
			switch sig {
			case syscall.SIGHUP:
				return nil
			default:
				s.Stop()
				return nil
			}
		}
	}
}

func (s *service) initFlags() {
	flag.StringVar(&s.env, "app-env", DevEnv, "Env for service. Ex: dev | stg | prd")

	for _, subService := range s.subServices {
		subService.InitFlags()
	}

	for _, dbService := range s.initServices {
		dbService.InitFlags()
	}

	for _, config := range s.configureServices {
		config.InitFlags()
	}
}

// Run service and its components at the same time
func (s *service) run() <-chan error {
	c := make(chan error, 1)

	// Start all services
	for _, subService := range s.subServices {
		go func(subSv Runnable) { c <- subSv.Run() }(subService)
	}

	return c
}

// Stop service and stop its components at the same time
func (s *service) Stop() {
	s.logger.Infoln("Stopping service...")
	stopChan := make(chan bool)
	for _, subService := range s.subServices {
		go func(subSv Runnable) { stopChan <- <-subSv.Stop() }(subService)
	}

	for _, dbSv := range s.initServices {
		go func(subSv Runnable) { stopChan <- <-subSv.Stop() }(dbSv)
	}

	for i := 0; i < len(s.subServices)+len(s.initServices); i++ {
		<-stopChan
	}

	//s.stopFunc()
	s.logger.Infoln("service stopped")
}

func (s *service) RunFunction(fn Function) error {
	return fn(s)
}

func (s *service) HTTPServer() HttpServer {
	return s.httpServer
}

func (s *service) Logger(prefix string) logger.Logger {
	return logger.GetCurrent().GetLogger(prefix)
}

func (s *service) OutEnv() {
	s.cmdLine.GetSampleEnvs()
}

func (s *service) RouteTable() {
	routes := s.HTTPServer().Routes()
	data := make([][]string, 0)
	for _, route := range routes {
		for _, ro := range route {
			r := []string{ro.Path, ro.Method, GetListNameHandler(ro.Handlers)}
			data = append(data, r)
		}

	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetHeader([]string{"Path", "Method", "Handler"})
	table.AppendBulk(data) // Add Bulk Data
	table.Render()
}

func (s *service) parseFlags() {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}

	_, err := os.Stat(envFile)
	if err == nil {
		err := godotenv.Load(envFile)
		if err != nil {
			s.logger.Fatalf("Loading env(%s): %s", envFile, err.Error())
		}
	} else if envFile != ".env" {
		s.logger.Fatalf("Loading env(%s): %s", envFile, err.Error())
	}

	s.cmdLine.Parse([]string{})
}

// Service must have a name for service discovery and logging/monitoring
func WithName(name string) Option {
	return func(s *service) { s.name = name }
}

// Every deployment needs a specific version
func WithVersion(version string) Option {
	return func(s *service) { s.version = version }
}

// Service will write log data to file with this option
func WithFileLogger() Option {
	return func(s *service) {
		logger.InitServLogger(true)
	}
}

// Add Runnable component to SDK
// These components will run parallel in when service run
func WithRunnable(r Runnable) Option {
	return func(s *service) { s.subServices = append(s.subServices, r) }
}

// Add init component to SDK
// These components will run sequentially before service run
func WithInitRunnable(r PrefixRunnable) Option {
	return func(s *service) {
		if _, ok := s.initServices[r.GetPrefix()]; ok {
			log.Fatal(fmt.Sprintf("prefix %s is duplicated", r.GetPrefix()))
		}

		s.initServices[r.GetPrefix()] = r
	}
}

// Add init component to SDK, example elasticsearch, .... that is third party service
// These components will init config for service from flag and return service in plugin
// Behind, it can init config, example get key jwt, key env
func WithInitConfig(r PrefixConfigure) Option {
	return func(s *service) {
		if _, ok := s.configureServices[r.GetPrefix()]; ok {
			log.Fatal(fmt.Sprintf("prefix %s is duplicated", r.GetPrefix()))
		}

		s.configureServices[r.GetPrefix()] = r
	}
}

func (s *service) Get(prefix string) (interface{}, bool) {
	is, ok := s.initServices[prefix]

	if !ok {
		if is, ok := s.configureServices[prefix]; ok {
			return is.Get(), true
		}

		return nil, ok
	}

	return is.Get(), true
}

func (s *service) MustGet(prefix string) interface{} {
	db, ok := s.Get(prefix)

	if !ok {
		panic(fmt.Sprintf("can not get %s\n", prefix))
	}

	return db
}

func (s *service) Env() string { return s.env }

func GetListNameHandler(hdls []fiber.Handler) string {
	res := ""
	for _, hdl := range hdls {
		res += util.GetFunctionName(hdl)
		res += ","
	}
	return res
}
