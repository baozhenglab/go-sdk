package httpserver

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/baozhenglab/go-sdk/v2/httpserver/middleware"
	"github.com/baozhenglab/go-sdk/v2/logger"
	"github.com/gofiber/fiber/v2"
	logfiber "github.com/gofiber/fiber/v2/middleware/logger"
)

var (
	fiberMode     string
	fiberNoLogger bool
	defaultPort   = 3000
)

type Config struct {
	Port           int    `json:"http_port"`
	BindAddr       string `json:"http_bind_addr"`
	FiberNoDefault bool   `json:"http_no_default"`
}

type GinService interface {
	// block until ready
	Port() int
	isFiberService()
}

type fiberService struct {
	Config
	isEnabled   bool
	name        string
	logger      logger.Logger
	app         *fiber.App
	mu          *sync.Mutex
	handlers    []func(*fiber.App)
	middlewares []fiber.Handler
	//registeredID  string
	//registryAgent registry.Agent
}

func New(name string) *fiberService {
	return &fiberService{
		name:        name,
		mu:          &sync.Mutex{},
		handlers:    []func(*fiber.App){},
		middlewares: []fiber.Handler{},
	}
}

func (fs *fiberService) Name() string {
	return fs.name + "-fiber"
}

func (fs *fiberService) InitFlags() {
	prefix := "fiber"
	flag.IntVar(&fs.Config.Port, prefix+"Port", defaultPort, "fiber server Port. If 0 => get a random Port")
	flag.StringVar(&fs.BindAddr, prefix+"addr", "", "fiber server bind address")
	flag.StringVar(&fiberMode, "fiber-mode", "", "fiber mode")
	flag.BoolVar(&fiberNoLogger, "fiber-no-logger", false, "disable default fiber logger middleware")
}

func (fs *fiberService) Configure() error {
	fs.logger = logger.GetCurrent().GetLogger("fiber")

	// if fiberMode == "release" {
	// 	gin.SetMode(gin.ReleaseMode)
	// }

	fs.logger.Debug("init fiber engine...")
	fs.app = fiber.New()
	for _, m := range fs.middlewares {
		fs.app.Use(m)
	}
	if !fs.FiberNoDefault {
		if !fiberNoLogger {
			fs.app.Use(logfiber.New())
		}
		//gs.router.Use(gin.Recovery())
		fs.app.Use(middleware.PanicLogger())
	}
	return nil
}

func formatBindAddr(s string, p int) string {
	if strings.Contains(s, ":") && !strings.Contains(s, "[") {
		s = "[" + s + "]"
	}
	return fmt.Sprintf("%s:%d", s, p)
}

func (fs *fiberService) Run() error {
	if !fs.isEnabled {
		return nil
	}

	if err := fs.Configure(); err != nil {
		return err
	}

	for _, hdl := range fs.handlers {
		hdl(fs.app)
	}

	addr := formatBindAddr(fs.BindAddr, fs.Config.Port)
	fs.logger.Debugf("start listen tcp %s...", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fs.logger.Fatalf("failed to listen: %v", err)
	}

	fs.Config.Port = getPort(lis)

	fs.logger.Infof("listen on %s...", lis.Addr().String())

	err = fs.app.Listener(lis)

	if err != nil && err == http.ErrServerClosed {
		return nil
	}
	return err
}

func getPort(lis net.Listener) int {
	addr := lis.Addr()
	tcp, _ := net.ResolveTCPAddr(addr.Network(), addr.String())
	return tcp.Port
}

func (fs *fiberService) Port() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.Config.Port
}

func (fs *fiberService) Stop() <-chan bool {
	c := make(chan bool)

	go func() {
		if fs.app != nil {
			_ = fs.app.Shutdown()
		}
		c <- true
	}()
	return c
}

func (fs *fiberService) URI() string {
	return formatBindAddr(fs.BindAddr, fs.Config.Port)
}

func (fs *fiberService) AddHandler(hdl func(*fiber.App)) {
	fs.isEnabled = true
	fs.handlers = append(fs.handlers, hdl)
}

func (fs *fiberService) AddMiddleware(hdl fiber.Handler) {
	fs.middlewares = append(fs.middlewares, hdl)
}

func (fs *fiberService) Reload(config Config) error {
	fs.Config = config
	<-fs.Stop()
	return fs.Run()
}

func (fs *fiberService) GetConfig() Config {
	return fs.Config
}

func (fs *fiberService) IsRunning() bool {
	return fs.app != nil
}

func (fs *fiberService) Routes() [][]*fiber.Route {
	fs.app = fiber.New()
	for _, hdl := range fs.handlers {
		hdl(fs.app)
	}
	return fs.app.Stack()
}
