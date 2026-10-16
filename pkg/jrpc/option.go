package jrpc

type Option func(*Server)

func WithLog(l ILog) Option {
	return func(s *Server) {
		s.ilog = l
	}
}

func WithIdentityResolver(resolver IdentityResolver) Option {
	return func(s *Server) {
		s.identityResolver = resolver
	}
}

// WithErrorMapper replaces the default application-error to RPC-error mapping.
func WithErrorMapper(mapper ErrorMapper) Option {
	return func(s *Server) {
		s.errorMapper = mapper
	}
}

// WithErrorLocalizer translates public business error codes for each request.
// It is used by the default mapper.
func WithErrorLocalizer(localizer ErrorLocalizer) Option {
	return func(s *Server) {
		s.errorLocalizer = localizer
	}
}

func WithAuth(auth AuthHandler) Option {
	return WithIdentityResolver(auth)
}

func WithRecoveryHandler(fn RecoveryHandler) Option {
	return func(s *Server) {
		s.recoverHandler = fn
	}
}

func WithDefaultAccess(mode AccessMode) Option {
	return func(s *Server) {
		s.defaultAccess = mode
	}
}

func WithMethodHyphen(hyphen string) Option { return func(s *Server) { s.services.setHyphen(hyphen) } }
