package main

import "github.com/go-chi/chi/middleware"

func (s *Server) MountHandlers() {
	// Mount all Middleware here
	//s.Router.Use(LoggerMiddleware(s.logger))
	s.Router.Use(middleware.Logger)
	//s.Router.Use(middleware.Recoverer)
	//s.Router.Use(middleware.CleanPath)
	s.Router.Use(middleware.Heartbeat("/ping"))

	// Mount all handlers here
	s.Router.Post("/sendsms", AddSMSToQueue)

}
