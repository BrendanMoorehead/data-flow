package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/api"
	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/identity"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
	"github.com/BrendanMoorehead/data-flow/internal/provider/aggregator"
	"github.com/BrendanMoorehead/data-flow/internal/provider/draftkings"
	"github.com/BrendanMoorehead/data-flow/internal/provider/fanduel"
	"github.com/BrendanMoorehead/data-flow/internal/ratelimit"
	"github.com/BrendanMoorehead/data-flow/internal/resolve"
	"github.com/BrendanMoorehead/data-flow/internal/sim"
	"github.com/BrendanMoorehead/data-flow/internal/source"
	"github.com/BrendanMoorehead/data-flow/internal/store"
	"github.com/BrendanMoorehead/data-flow/internal/syncer"
)

const (
	simDriftInterval = 1500 * time.Millisecond
	resolveInterval  = time.Second
	shutdownTimeout  = 5 * time.Second
)

type options struct {
	databasePath   string
	apiAddress     string
	aggregatorAddr string
	draftKingsAddr string
	fanDuelAddr    string
	simAdminAddr   string
	seed           uint64
	calm           bool
}

func main() {
	opts := parseOptions()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, opts, logger); err != nil {
		logger.Error("data-flow stopped", "error", err)
		os.Exit(1)
	}
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.databasePath, "db", "data-flow.db", "SQLite database file")
	flag.StringVar(&opts.apiAddress, "addr", "127.0.0.1:8080", "address for the read API")
	flag.StringVar(&opts.aggregatorAddr, "aggregator-addr", "127.0.0.1:9101", "address for the simulated aggregator")
	flag.StringVar(&opts.draftKingsAddr, "draftkings-addr", "127.0.0.1:9102", "address for the simulated DraftKings feed")
	flag.StringVar(&opts.fanDuelAddr, "fanduel-addr", "127.0.0.1:9103", "address for the simulated FanDuel feed")
	flag.StringVar(&opts.simAdminAddr, "sim-admin-addr", "127.0.0.1:9100", "address for the simulator's scenario admin API")
	flag.Uint64Var(&opts.seed, "seed", 42, "seed for simulated price movement and background problems")
	flag.BoolVar(&opts.calm, "calm", false, "turn off the simulator's background problems")
	flag.Parse()
	return opts
}

func utcNow() time.Time {
	return time.Now().UTC()
}

func sourceRegistry() source.Registry {
	return source.NewRegistry(
		source.Config{
			ID:                     canonical.SourceDraftKingsDirect,
			Kind:                   source.KindDirect,
			PollInterval:           3 * time.Second,
			StaleAfter:             12 * time.Second,
			RequestTimeout:         2 * time.Second,
			CatalogRefreshInterval: 30 * time.Second,
			Workers:                3,
			RequestsPerSecond:      10,
			RequestBurst:           5,
		},
		source.Config{
			ID:                     canonical.SourceFanDuelDirect,
			Kind:                   source.KindDirect,
			PollInterval:           3 * time.Second,
			StaleAfter:             12 * time.Second,
			RequestTimeout:         2 * time.Second,
			CatalogRefreshInterval: 30 * time.Second,
			Workers:                1,
			RequestsPerSecond:      5,
			RequestBurst:           2,
		},
		source.Config{
			ID:                     canonical.SourceAggregator,
			Kind:                   source.KindAggregator,
			PollInterval:           8 * time.Second,
			StaleAfter:             30 * time.Second,
			RequestTimeout:         4 * time.Second,
			CatalogRefreshInterval: time.Minute,
			Workers:                1,
			RequestsPerSecond:      2,
			RequestBurst:           2,
		},
	)
}

func httpClientFor(config source.Config) *http.Client {
	budget := ratelimit.NewBudget(config.RequestsPerSecond, config.RequestBurst)
	return &http.Client{
		Timeout:   config.RequestTimeout,
		Transport: &ratelimit.Transport{Base: http.DefaultTransport, Budget: budget},
	}
}

func run(ctx context.Context, opts options, logger *slog.Logger) error {
	db, err := store.Open(ctx, opts.databasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	seed, err := identity.LoadDefaultSeed()
	if err != nil {
		return fmt.Errorf("load team fixtures: %w", err)
	}
	if err := db.SeedTeams(ctx, seed.Teams, seed.Aliases); err != nil {
		return err
	}

	simulator := sim.New(sim.Options{
		Seed:               opts.seed,
		BackgroundProblems: !opts.calm,
		Now:                utcNow,
		DashboardOrigin:    "http://" + opts.apiAddress,
	})
	sources := sourceRegistry()
	providers := []provider.Provider{
		aggregator.New("http://"+opts.aggregatorAddr, httpClientFor(sources[canonical.SourceAggregator])),
		draftkings.New("http://"+opts.draftKingsAddr, httpClientFor(sources[canonical.SourceDraftKingsDirect])),
		fanduel.New("http://"+opts.fanDuelAddr, httpClientFor(sources[canonical.SourceFanDuelDirect])),
	}
	synchronizer := syncer.New(syncer.Dependencies{
		Providers: providers,
		Sources:   sources,
		Events:    identity.NewResolver(db, logger),
		Store:     db,
		Logger:    logger,
		Now:       utcNow,
	})
	rebuilder := resolve.NewRebuilder(db, sources, logger, utcNow)
	apiServer := api.NewServer(api.ServerConfig{
		Store:       db,
		Sources:     sources,
		Now:         utcNow,
		SimAdminURL: "http://" + opts.simAdminAddr,
	})

	servers := []namedServer{
		{name: "simulated aggregator", address: opts.aggregatorAddr, handler: simulator.AggregatorHandler()},
		{name: "simulated draftkings", address: opts.draftKingsAddr, handler: simulator.DraftKingsHandler()},
		{name: "simulated fanduel", address: opts.fanDuelAddr, handler: simulator.FanDuelHandler()},
		{name: "simulator admin", address: opts.simAdminAddr, handler: simulator.AdminHandler()},
		{name: "api", address: opts.apiAddress, handler: apiServer.Handler()},
	}
	listeners, err := listenAll(servers)
	if err != nil {
		return err
	}

	var running backgroundTasks
	for index, server := range servers {
		running.start(func() { serveUntilDone(ctx, server, listeners[index], logger) })
	}
	running.start(func() { simulator.Run(ctx, simDriftInterval) })
	running.start(func() { synchronizer.Run(ctx) })
	running.start(func() { rebuilder.Run(ctx, resolveInterval) })

	logger.Info("data-flow running",
		"dashboard", "http://"+opts.apiAddress+"/",
		"api", "http://"+opts.apiAddress,
		"sim_admin", "http://"+opts.simAdminAddr,
		"db", opts.databasePath,
		"background_problems", !opts.calm)
	running.wait()
	return nil
}

type namedServer struct {
	name    string
	address string
	handler http.Handler
}

func listenAll(servers []namedServer) ([]net.Listener, error) {
	listeners := make([]net.Listener, 0, len(servers))
	for _, server := range servers {
		listener, err := net.Listen("tcp", server.address)
		if err != nil {
			closeAll(listeners)
			return nil, fmt.Errorf("listen for %s on %s: %w", server.name, server.address, err)
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

func closeAll(listeners []net.Listener) {
	for _, listener := range listeners {
		listener.Close()
	}
}

func serveUntilDone(ctx context.Context, server namedServer, listener net.Listener, logger *slog.Logger) {
	httpServer := &http.Server{Handler: server.handler, ReadHeaderTimeout: 5 * time.Second}
	go shutdownOnDone(ctx, httpServer)
	logger.Info("listening", "server", server.name, "address", server.address)
	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "server", server.name, "error", err)
	}
}

func shutdownOnDone(ctx context.Context, httpServer *http.Server) {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	httpServer.Shutdown(shutdownCtx)
}

type backgroundTasks struct {
	group sync.WaitGroup
}

func (b *backgroundTasks) start(task func()) {
	b.group.Add(1)
	go func() {
		defer b.group.Done()
		task()
	}()
}

func (b *backgroundTasks) wait() {
	b.group.Wait()
}
