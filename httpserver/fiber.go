package httpserver

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/baozhenglab/go-sdk/v2/httpserver/middleware"
	"github.com/baozhenglab/go-sdk/v2/logger"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
	logfiber "github.com/gofiber/fiber/v2/middleware/logger"
	"go.opencensus.io/plugin/ochttp"
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
	svr         *myHttpServer
	router      *fiber.App
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
	fs.router = fiber.New()
	for _, m := range fs.middlewares {
		fs.router.Use(m)
	}
	if !fs.FiberNoDefault {
		if !fiberNoLogger {
			fs.router.Use(logfiber.New())
		}
		//gs.router.Use(gin.Recovery())
		fs.router.Use(middleware.PanicLogger())
	}
	och := &ochttp.Handler{
		Handler: fs.router,
	}

	fs.svr = &myHttpServer{
		Server: http.Server{Handler: och},
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
		hdl(fs.router)
	}

	addr := formatBindAddr(fs.BindAddr, fs.Config.Port)
	fs.logger.Debugf("start listen tcp %s...", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fs.logger.Fatalf("failed to listen: %v", err)
	}

	fs.Config.Port = getPort(lis)

	fs.logger.Infof("listen on %s...", lis.Addr().String())

	err = fs.svr.Serve(lis)

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

func (gs *ginService) Port() int {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.Config.Port
}

func (gs *ginService) Stop() <-chan bool {
	c := make(chan bool)

	go func() {
		if gs.svr != nil {
			_ = gs.svr.Shutdown(context.Background())
		}
		c <- true
	}()
	return c
}

func (gs *ginService) URI() string {
	return formatBindAddr(gs.BindAddr, gs.Config.Port)
}

func (gs *ginService) AddHandler(hdl func(*gin.Engine)) {
	gs.isEnabled = true
	gs.handlers = append(gs.handlers, hdl)
}

func (gs *ginService) AddMiddleware(hdl gin.HandlerFunc) {
	gs.middlewares = append(gs.middlewares, hdl)
}

func (gs *ginService) Reload(config Config) error {
	gs.Config = config
	<-gs.Stop()
	return gs.Run()
}

func (gs *ginService) GetConfig() Config {
	return gs.Config
}

func (gs *ginService) IsRunning() bool {
	return gs.svr != nil
}

func (gs *ginService) Routes() []gin.RouteInfo {
	gin.SetMode(gin.ReleaseMode)
	gs.router = gin.New()
	for _, hdl := range gs.handlers {
		hdl(gs.router)
	}
	return gs.router.Routes()
}
