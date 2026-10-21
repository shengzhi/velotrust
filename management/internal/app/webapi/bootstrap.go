package webapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"velotrust/management/internal/config"
	enrollmentHandler "velotrust/management/internal/handler/enrollment"
	"velotrust/management/internal/handler/iam"
	ipamHandler "velotrust/management/internal/handler/ipam"
	nodeHandler "velotrust/management/internal/handler/node"
	siteHandler "velotrust/management/internal/handler/site"
	"velotrust/management/internal/lib/cache"
	"velotrust/management/internal/lib/logger"
	"velotrust/management/internal/middleware"
	appEnrollmentRepo "velotrust/management/internal/repository/enrollment"
	appIAMRepo "velotrust/management/internal/repository/iam"
	appIPAMRepo "velotrust/management/internal/repository/ipam"
	appNodeRepo "velotrust/management/internal/repository/node"
	appSiteRepo "velotrust/management/internal/repository/site"
	appEnrollmentService "velotrust/management/internal/service/enrollment"
	appIAMService "velotrust/management/internal/service/iam"
	appIPAMService "velotrust/management/internal/service/ipam"
	appNodeService "velotrust/management/internal/service/node"
	appSiteService "velotrust/management/internal/service/site"
	management "velotrust/management/runtime"
	"velotrust/pkg/ioc"
	"velotrust/pkg/jrpc"
	"velotrust/shared/repository/database"
)

var dependencyRegistrationMu sync.Mutex
var dependencyRegistered bool

// Build initializes shared dependencies through the template IOC container
// and composes the existing Management runtime with the IAM JRPC registrar.
func Build(conf *config.Config, listenAddr, version string) (*management.Server, *database.Database, func() error, error) {
	if conf == nil {
		return nil, nil, nil, errors.New("Management configuration is required")
	}
	if conf.Database.Driver == "" || conf.Database.DSN == "" {
		return nil, nil, nil, errors.New("database.driver and database.dsn are required")
	}
	driver := strings.ToLower(strings.TrimSpace(conf.Database.Driver))
	if driver != "postgres" && driver != "postgresql" && driver != "pgx" {
		return nil, nil, nil, fmt.Errorf("IAM currently requires PostgreSQL; unsupported database driver %q", conf.Database.Driver)
	}
	logger.InitLogger(conf.App.LogLevel, conf.App.LogFormat)
	db, err := database.NewDatabase("postgres", conf.Database.DSN)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("initialize shared database: %w", err)
	}
	db.Engine().ShowSQL(conf.Database.ShowSQL)
	if conf.Database.MaxOpenConnections > 0 {
		db.Engine().DB().SetMaxOpenConns(conf.Database.MaxOpenConnections)
	}
	if conf.Database.MaxIdleConnections > 0 {
		db.Engine().DB().SetMaxIdleConns(conf.Database.MaxIdleConnections)
	}
	if conf.Database.ConnectionMaxLifetimeSeconds > 0 {
		db.Engine().DB().SetConnMaxLifetime(time.Duration(conf.Database.ConnectionMaxLifetimeSeconds) * time.Second)
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Ping(pingCtx); err != nil {
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("verify shared database: %w", err)
	}
	var ceremonyStore cache.OneTimeStore = cache.NewLocalOneTimeStore()
	var ceremonyCloser io.Closer
	if conf.Cache.Redis.Addr != "" {
		redisStore, redisErr := cache.NewRedisOneTimeStore(conf.Cache.Redis.Addr, conf.Cache.Redis.Password, conf.Cache.Redis.DB)
		if redisErr != nil {
			_ = db.Close()
			return nil, nil, nil, redisErr
		}
		pingCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		redisErr = redisStore.Ping(pingCtx)
		cancel()
		if redisErr != nil {
			_ = redisStore.Close()
			_ = db.Close()
			return nil, nil, nil, fmt.Errorf("verify configured Redis ceremony store: %w", redisErr)
		}
		ceremonyStore, ceremonyCloser = redisStore, redisStore
	}
	if err := registerDependencies(conf, db, ceremonyStore); err != nil {
		if ceremonyCloser != nil {
			_ = ceremonyCloser.Close()
		}
		_ = db.Close()
		return nil, nil, nil, err
	}
	service, err := ioc.GetObject[*appIAMService.Service]()
	if err != nil {
		if ceremonyCloser != nil {
			_ = ceremonyCloser.Close()
		}
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("resolve IAM service from IOC: %w", err)
	}
	resolver := middleware.SessionIdentityResolver(service)
	enrollmentService, err := ioc.GetObject[*appEnrollmentService.Service]()
	if err != nil {
		if ceremonyCloser != nil {
			_ = ceremonyCloser.Close()
		}
		_ = service.Close()
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("resolve enrollment service from IOC: %w", err)
	}
	ipamService, err := ioc.GetObject[*appIPAMService.Service]()
	if err != nil {
		if ceremonyCloser != nil {
			_ = ceremonyCloser.Close()
		}
		_ = service.Close()
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("resolve IPAM service from IOC: %w", err)
	}
	siteService, err := ioc.GetObject[*appSiteService.Service]()
	if err != nil {
		if ceremonyCloser != nil {
			_ = ceremonyCloser.Close()
		}
		_ = service.Close()
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("resolve network-site service from IOC: %w", err)
	}
	nodeService, err := ioc.GetObject[*appNodeService.Service]()
	if err != nil {
		if ceremonyCloser != nil {
			_ = ceremonyCloser.Close()
		}
		_ = service.Close()
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("resolve virtual-node service from IOC: %w", err)
	}
	server, err := management.New(management.Config{
		ListenAddr: listenAddr, Version: version, IdentityResolver: resolver,
		TrustedProxies: conf.HTTP.TrustedProxies,
		HTTPMiddleware: middleware.CORS(middleware.CORSConfig{
			AllowedOrigins: conf.HTTP.CORS.AllowedOrigins,
			AllowedMethods: conf.HTTP.CORS.AllowedMethods,
			AllowedHeaders: conf.HTTP.CORS.AllowedHeaders,
			MaxAgeSeconds:  conf.HTTP.CORS.MaxAgeSeconds,
		}),
	}, func(rpc *jrpc.Server) error {
		if err := iam.Register(rpc, service); err != nil {
			return err
		}
		if err := enrollmentHandler.Register(rpc, enrollmentService, service); err != nil {
			return err
		}
		if err := ipamHandler.Register(rpc, ipamService, service); err != nil {
			return err
		}
		if err := siteHandler.Register(rpc, siteService, service); err != nil {
			return err
		}
		if err := nodeHandler.Register(rpc, nodeService, service); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if ceremonyCloser != nil {
			_ = ceremonyCloser.Close()
		}
		_ = service.Close()
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("compose Management JRPC server: %w", err)
	}
	closeResources := func() error {
		var closeErr error
		if ceremonyCloser != nil {
			closeErr = errors.Join(closeErr, ceremonyCloser.Close())
		}
		closeErr = errors.Join(closeErr, service.Close())
		return closeErr
	}
	return server, db, closeResources, nil
}

func registerDependencies(conf *config.Config, db *database.Database, ceremonyStore cache.OneTimeStore) error {
	dependencyRegistrationMu.Lock()
	defer dependencyRegistrationMu.Unlock()
	if dependencyRegistered {
		return errors.New("Management IOC dependencies are already registered; build one Management runtime per process")
	}
	ioc.RegistObjectFunc[config.Config](func() (*config.Config, error) { return conf, nil }, ioc.Singleton)
	ioc.RegistObjectFunc[database.Database](func() (*database.Database, error) { return db, nil }, ioc.Singleton)
	ioc.RegistInterfaceFunc[appIAMRepo.Store, appIAMRepo.Repository](func() (*appIAMRepo.Repository, error) {
		sharedDB, err := ioc.GetObject[*database.Database]()
		if err != nil {
			return nil, err
		}
		return appIAMRepo.New(sharedDB)
	}, ioc.Singleton)
	ioc.RegistObjectFunc[appIAMService.Service](func() (*appIAMService.Service, error) {
		store, err := ioc.GetObject[appIAMRepo.Store]()
		if err != nil {
			return nil, err
		}
		loaded, err := ioc.GetObject[*config.Config]()
		if err != nil {
			return nil, err
		}
		idleTTL := time.Duration(loaded.Auth.Session.IdleTTLMinutes) * time.Minute
		absoluteTTL := time.Duration(loaded.Auth.Session.AbsoluteTTLMinutes) * time.Minute
		return appIAMService.New(store, appIAMService.Options{
			SessionIdleTTL: idleTTL, SessionAbsoluteTTL: absoluteTTL,
			JWTSigningKey:     []byte(loaded.Auth.JWTSigningKey),
			ThrottleHMACKey:   []byte(loaded.Auth.ThrottleHMACKey),
			TOTPEncryptionKey: []byte(loaded.Auth.TOTPEncryptionKey), TOTPKeyVersion: loaded.Auth.TOTPKeyVersion,
			WebAuthnRPID: loaded.Auth.WebAuthn.RPID, WebAuthnName: loaded.Auth.WebAuthn.DisplayName,
			WebAuthnOrigins: loaded.Auth.WebAuthn.Origins, CeremonyStore: ceremonyStore,
		})
	}, ioc.Singleton)
	ioc.RegistInterfaceFunc[appEnrollmentRepo.Store, appEnrollmentRepo.Repository](func() (*appEnrollmentRepo.Repository, error) {
		sharedDB, err := ioc.GetObject[*database.Database]()
		if err != nil {
			return nil, err
		}
		return appEnrollmentRepo.New(sharedDB)
	}, ioc.Singleton)
	ioc.RegistObjectFunc[appEnrollmentService.Service](func() (*appEnrollmentService.Service, error) {
		store, err := ioc.GetObject[appEnrollmentRepo.Store]()
		if err != nil {
			return nil, err
		}
		permissions, err := ioc.GetObject[*appIAMService.Service]()
		if err != nil {
			return nil, err
		}
		networks, err := ioc.GetObject[appIPAMRepo.Store]()
		if err != nil {
			return nil, err
		}
		return appEnrollmentService.New(store, permissions, networks)
	}, ioc.Singleton)
	ioc.RegistInterfaceFunc[appIPAMRepo.Store, appIPAMRepo.Repository](func() (*appIPAMRepo.Repository, error) {
		sharedDB, err := ioc.GetObject[*database.Database]()
		if err != nil {
			return nil, err
		}
		return appIPAMRepo.New(sharedDB)
	}, ioc.Singleton)
	ioc.RegistObjectFunc[appIPAMService.Service](func() (*appIPAMService.Service, error) {
		store, err := ioc.GetObject[appIPAMRepo.Store]()
		if err != nil {
			return nil, err
		}
		permissions, err := ioc.GetObject[*appIAMService.Service]()
		if err != nil {
			return nil, err
		}
		return appIPAMService.New(store, permissions)
	}, ioc.Singleton)
	ioc.RegistInterfaceFunc[appSiteRepo.Store, appSiteRepo.Repository](func() (*appSiteRepo.Repository, error) {
		sharedDB, err := ioc.GetObject[*database.Database]()
		if err != nil {
			return nil, err
		}
		return appSiteRepo.New(sharedDB)
	}, ioc.Singleton)
	ioc.RegistObjectFunc[appSiteService.Service](func() (*appSiteService.Service, error) {
		store, err := ioc.GetObject[appSiteRepo.Store]()
		if err != nil {
			return nil, err
		}
		permissions, err := ioc.GetObject[*appIAMService.Service]()
		if err != nil {
			return nil, err
		}
		natService, err := ioc.GetObject[*appIPAMService.Service]()
		if err != nil {
			return nil, err
		}
		return appSiteService.New(store, permissions, natService)
	}, ioc.Singleton)
	ioc.RegistInterfaceFunc[appNodeRepo.Store, appNodeRepo.Repository](func() (*appNodeRepo.Repository, error) {
		sharedDB, err := ioc.GetObject[*database.Database]()
		if err != nil {
			return nil, err
		}
		return appNodeRepo.New(sharedDB)
	}, ioc.Singleton)
	ioc.RegistObjectFunc[appNodeService.Service](func() (*appNodeService.Service, error) {
		store, err := ioc.GetObject[appNodeRepo.Store]()
		if err != nil {
			return nil, err
		}
		permissions, err := ioc.GetObject[*appIAMService.Service]()
		if err != nil {
			return nil, err
		}
		return appNodeService.New(store, permissions)
	}, ioc.Singleton)
	dependencyRegistered = true
	return nil
}
