// Package main is the composition root for granola-mcp.
// All dependencies are wired here — no service locator, no global state.
// This is the only place that knows about all layers simultaneously.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	annotationapp "github.com/felixgeelhaar/granola-mcp/internal/application/annotation"
	authapp "github.com/felixgeelhaar/granola-mcp/internal/application/auth"
	embeddingapp "github.com/felixgeelhaar/granola-mcp/internal/application/embedding"
	exportapp "github.com/felixgeelhaar/granola-mcp/internal/application/export"
	meetingapp "github.com/felixgeelhaar/granola-mcp/internal/application/meeting"
	workspaceapp "github.com/felixgeelhaar/granola-mcp/internal/application/workspace"
	domain "github.com/felixgeelhaar/granola-mcp/internal/domain/meeting"
	infraauth "github.com/felixgeelhaar/granola-mcp/internal/infrastructure/auth"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/cache"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/config"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/events"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/granola"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/localstore"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/outbox"
	infraPolicy "github.com/felixgeelhaar/granola-mcp/internal/infrastructure/policy"
	"github.com/felixgeelhaar/granola-mcp/internal/infrastructure/resilience"
	"github.com/felixgeelhaar/granola-mcp/internal/interfaces/cli"
	mcpiface "github.com/felixgeelhaar/granola-mcp/internal/interfaces/mcp"
	_ "github.com/mattn/go-sqlite3"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Set version info for the CLI
	cli.Version = version
	cli.Commit = commit
	cli.Date = date

	// Load configuration (file defaults + env overrides)
	cfg := config.Load()

	// --- Infrastructure Layer ---

	// HTTP client for Granola API
	httpClient := &http.Client{Timeout: cfg.Resilience.Timeout}

	// Granola API client (anti-corruption layer)
	granolaClient := granola.NewClient(cfg.Granola.APIURL, httpClient, cfg.Granola.APIToken)

	// Repository: Granola API → domain.Repository
	granolaRepo := granola.NewRepository(granolaClient)

	// Resilience decorator (circuit breaker, timeout, retry, rate limit)
	resilientRepo := resilience.NewResilientRepository(granolaRepo, resilience.Config{
		Timeout:          cfg.Resilience.Timeout,
		MaxRetries:       cfg.Resilience.Retry.MaxAttempts,
		RetryDelay:       cfg.Resilience.Retry.InitialDelay,
		RetryMaxDelay:    cfg.Resilience.Retry.MaxDelay,
		FailureThreshold: cfg.Resilience.CircuitBreaker.FailureThreshold,
		SuccessThreshold: cfg.Resilience.CircuitBreaker.SuccessThreshold,
		HalfOpenTimeout:  cfg.Resilience.CircuitBreaker.HalfOpenTimeout,
		RateLimit:        cfg.Resilience.RateLimit.Rate,
		RateBurst:        cfg.Resilience.RateLimit.Rate * 2,
		RateInterval:     cfg.Resilience.RateLimit.Interval,
	})
	defer func() { _ = resilientRepo.Close() }()

	// Cache decorator (SQLite local cache)
	var repo domain.Repository = resilientRepo
	if cfg.Cache.Enabled {
		cacheDir := cfg.Cache.Dir
		if err := os.MkdirAll(cacheDir, 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot create cache dir: %v\n", err)
		} else {
			dbPath := filepath.Join(cacheDir, "cache.db")
			db, err := sql.Open("sqlite3", dbPath)
			if err == nil {
				cachedRepo, cacheErr := cache.NewCachedRepository(resilientRepo, db, cfg.Cache.TTL)
				if cacheErr == nil {
					repo = cachedRepo
					defer func() { _ = db.Close() }()
				}
			}
		}
	}

	// Auth infrastructure
	homeDir, _ := os.UserHomeDir()
	tokenStore := infraauth.NewFileTokenStore(homeDir + "/.granola-mcp")
	authService := infraauth.NewService(tokenStore)

	// If we have a stored token, set it on the Granola client
	if cred, err := authService.Status(context.Background()); err == nil && cred.IsValid() {
		granolaClient.SetToken(cred.Token().AccessToken())
	}

	// Workspace repository
	wsRepo := granola.NewWorkspaceRepository(granolaClient)

	// Local store (SQLite for write-side: notes, action item overrides, outbox)
	localDir := cfg.Cache.Dir // Reuse cache dir for local store
	if err := os.MkdirAll(localDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot create local store dir: %v\n", err)
	}
	localDBPath := filepath.Join(localDir, "local.db")
	localDB, err := sql.Open("sqlite3", localDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot open local store: %v\n", err)
	} else {
		defer func() { _ = localDB.Close() }()
		if err := localstore.InitSchema(localDB); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot init local store schema: %v\n", err)
		}
	}

	// Local store repositories
	noteRepo := localstore.NewNoteRepository(localDB)
	writeRepo := localstore.NewWriteRepository(localDB)

	// Event infrastructure: inner dispatcher → outbox decorator
	innerDispatcher := events.NewDispatcher(nil) // notifier wired after MCP server creation
	outboxStore := outbox.NewSQLiteStore(localDB)
	var dispatcher domain.EventDispatcher = outbox.NewDispatcher(innerDispatcher, outboxStore)

	// --- Application Layer (Use Cases) ---

	listMeetings := meetingapp.NewListMeetings(repo)
	getMeeting := meetingapp.NewGetMeeting(repo)
	getTranscript := meetingapp.NewGetTranscript(repo)
	searchTranscripts := meetingapp.NewSearchTranscripts(repo)
	getActionItems := meetingapp.NewGetActionItems(repo)
	getMeetingStats := meetingapp.NewGetMeetingStats(repo)
	syncMeetings := meetingapp.NewSyncMeetings(repo)
	exportMeeting := exportapp.NewExportMeeting(repo)
	login := authapp.NewLogin(authService)
	checkStatus := authapp.NewCheckStatus(authService)
	listWorkspaces := workspaceapp.NewListWorkspaces(wsRepo)
	getWorkspace := workspaceapp.NewGetWorkspace(wsRepo)

	// Write use cases (Phase 3)
	addNote := annotationapp.NewAddNote(noteRepo, repo, dispatcher)
	listNotes := annotationapp.NewListNotes(noteRepo)
	deleteNote := annotationapp.NewDeleteNote(noteRepo, dispatcher)
	completeActionItem := meetingapp.NewCompleteActionItem(repo, writeRepo, dispatcher)
	updateActionItem := meetingapp.NewUpdateActionItem(repo, writeRepo, dispatcher)
	exportEmbeddings := embeddingapp.NewExportEmbeddings(repo, noteRepo)

	// --- Interfaces Layer ---

	// MCP server
	mcpServer := mcpiface.NewServer(cfg.MCP.ServerName, version, mcpiface.ServerOptions{
		ListMeetings:       listMeetings,
		GetMeeting:         getMeeting,
		GetTranscript:      getTranscript,
		SearchTranscripts:  searchTranscripts,
		GetActionItems:     getActionItems,
		GetMeetingStats:    getMeetingStats,
		ListWorkspaces:     listWorkspaces,
		GetWorkspace:       getWorkspace,
		AddNote:            addNote,
		ListNotes:          listNotes,
		DeleteNote:         deleteNote,
		CompleteActionItem: completeActionItem,
		UpdateActionItem:   updateActionItem,
		ExportEmbeddings:   exportEmbeddings,
	})

	// Policy middleware (wraps MCP server if policy file is configured)
	if cfg.Policy.Enabled && cfg.Policy.FilePath != "" {
		loadResult, policyErr := infraPolicy.LoadFromFile(cfg.Policy.FilePath)
		if policyErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot load policy file: %v\n", policyErr)
		} else {
			policyEngine := infraPolicy.NewEngine(loadResult)
			_ = mcpiface.NewPolicyMiddleware(mcpServer, policyEngine)
		}
	}

	// CLI dependencies
	deps := &cli.Dependencies{
		ListMeetings:       listMeetings,
		GetMeeting:         getMeeting,
		GetTranscript:      getTranscript,
		SearchTranscripts:  searchTranscripts,
		GetActionItems:     getActionItems,
		SyncMeetings:       syncMeetings,
		ExportMeeting:      exportMeeting,
		Login:              login,
		CheckStatus:        checkStatus,
		ListWorkspaces:     listWorkspaces,
		GetWorkspace:       getWorkspace,
		EventDispatcher:    dispatcher,
		MCPServer:          mcpServer,
		AddNote:            addNote,
		ListNotes:          listNotes,
		DeleteNote:         deleteNote,
		CompleteActionItem: completeActionItem,
		UpdateActionItem:   updateActionItem,
		ExportEmbeddings:   exportEmbeddings,
		Out:                os.Stdout,
	}

	// Execute CLI
	if err := cli.NewRootCmd(deps).Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
