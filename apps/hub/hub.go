package main

import (
	"flag"
	"fmt"

	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/config"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/handler"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/middleware"
	"github.com/cuteLittleDevil-ai/localmind-hub/apps/hub/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/hub.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// go-zero REST：鉴权中间件 + 设备/Agent 路由，依赖在 ServiceContext 内一次性连线。
	server := rest.MustNewServer(c.RestConf)
	server.Use(middleware.AuthMiddleware(c.Auth.DemoToken))
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting hub at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
