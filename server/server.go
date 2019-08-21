package server

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/go-ignite/ignite/config"
	"github.com/go-ignite/ignite/service"
)

var Set = wire.NewSet(wire.Struct(new(Options), "*"), New)

type Server struct {
	engine *gin.Engine
	opts   *Options
}

type Options struct {
	Config  config.Server
	Service *service.Service
}

func New(opts *Options) *Server {
	s := &Server{
		engine: gin.Default(),
		opts:   opts,
	}

	userRouter := s.engine.Group("/api/user")
	{
		userRouter.POST("/login", s.opts.Service.UserLogin)
		userRouter.POST("/register", s.opts.Service.UserRegister)

		userRouter.Use(s.opts.Service.Auth(false))
		{
			userRouter.PUT("/change_password", s.opts.Service.UserChangePassword)
			userRouter.GET("/sync", s.opts.Service.UserServicesSync)
			userRouter.POST("/services", s.opts.Service.CreateService)
			userRouter.GET("/services", s.opts.Service.GetUserServices)
			userRouter.GET("/services/options", s.opts.Service.GetServiceOptions)
			//authRouter.GET("/info", userHandler.UserInfoHandler)
		}
	}

	adminRouter := s.engine.Group("/api/admin")
	{
		adminRouter.POST("/login", s.opts.Service.AdminLogin)
		adminRouter.Use(s.opts.Service.Auth(true))
		{
			//user account related operations
			adminRouter.GET("/accounts", s.opts.Service.GetAccountList)
			adminRouter.PUT("/accounts/:id/reset_password", s.opts.Service.ResetAccountPassword)
			adminRouter.DELETE("/accounts/:id", s.opts.Service.DestroyAccount)
			// authRouter.PUT("/:id/stop", r.StopServiceHandler)
			// authRouter.PUT("/:id/start", r.StartServiceHandler)
			// authRouter.PUT("/:id/renew", r.RenewServiceHandler)

			// codes
			adminRouter.GET("/codes", s.opts.Service.GetInviteCodeList)
			adminRouter.DELETE("/codes/:id", s.opts.Service.RemoveInviteCode)
			adminRouter.POST("/codes_batch", s.opts.Service.GenerateInviteCodes)
			adminRouter.DELETE("/codes_prune", s.opts.Service.PruneInviteCodes)

			// nodes
			adminRouter.GET("/nodes", s.opts.Service.GetAllNodes)
			adminRouter.POST("/nodes", s.opts.Service.AddNode)
			adminRouter.PUT("/nodes/:id", s.opts.Service.UpdateNode)
			adminRouter.DELETE("/nodes/:id", s.opts.Service.DeleteNode)

			// services
			adminRouter.GET("/services", s.opts.Service.GetServices)
			//authRouter.DELETE("/services/:id", adminHandler.RemoveService)
		}
	}

	return s
}

func (s *Server) Start() error {
	return s.engine.Run(s.opts.Config.Address)
}
