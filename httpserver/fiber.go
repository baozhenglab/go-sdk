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
	return gs.name + "-fiber"
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

	gs.logger.Debug("init gin engine...")
	gs.router = gin.New()
	if !gs.GinNoDefault {
		if !ginNoLogger {
			gs.router.Use(gin.Logger())
		}
		//gs.router.Use(gin.Recovery())
		gs.router.Use(middleware.PanicLogger())
	}
	for _, m := range gs.middlewares {
		gs.router.Use(m)
	}
	och := &ochttp.Handler{
		Handler: gs.router,
	}

	gs.svr = &myHttpServer{
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

func (gs *ginService) Run() error {
	if !gs.isEnabled {
		return nil
	}

	if err := gs.Configure(); err != nil {
		return err
	}

	for _, hdl := range gs.handlers {
		hdl(gs.router)
	}

	addr := formatBindAddr(gs.BindAddr, gs.Config.Port)
	gs.logger.Debugf("start listen tcp %s...", addr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		gs.logger.Fatalf("failed to listen: %v", err)
	}

	gs.Config.Port = getPort(lis)

	gs.logger.Infof("listen on %s...", lis.Addr().String())

	err = gs.svr.Serve(lis)

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
