package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"webscraper/config"
	"webscraper/database"
	"webscraper/repository"
	"webscraper/web"
	"webscraper/usecase"
)

const (
	configFile = "config.yaml"
	appName    = "WebScraper"
	version    = "1.0"
)

func main() {
	log.Printf("Starting %s v%s", appName, version)
	cfg, err := loadConfig()

	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}
	db, err := initializeDatabase(cfg)

	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()
	repo := repository.NewScrapingRepository(db)
	uc := usecase.NewScrapingUseCase(repo, cfg)
	server := web.NewServer(cfg.Server.Port, uc)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		log.Printf("Server URL: http://localhost:%s", cfg.Server.Port)

		if err := server.Start(); err != nil {
			log.Fatal("Failed to start server:", err)
		}
	}()
	sig := <-sigChan
	log.Printf("Received signal: %v. Shutting down gracefully...", sig)

	log.Printf("%s v%s shutdown complete", appName, version)
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(configFile)

	if err != nil {
		return nil, fmt.Errorf("error loading config file '%s': %w", configFile, err)
	}
	log.Printf("Configuration loaded successfully")
	log.Printf("- Server port: %s", cfg.Server.Port)
	log.Printf("- Database path: %s", cfg.Database.Path)
	log.Printf("- User agent: %s", cfg.Scraping.UserAgent)
	log.Printf("- Request timeout: %ds", cfg.Scraping.Timeout)
	log.Printf("- Max links: %d", cfg.Scraping.MaxLinks)
	log.Printf("- Max images: %d", cfg.Scraping.MaxImages)
	log.Printf("- Analytics enabled: %t", cfg.Features.EnableAnalytics)
	log.Printf("- Caching enabled: %t", cfg.Features.EnableCaching)

	return cfg, nil
}

func initializeDatabase(cfg *config.Config) (*database.SQLiteDB, error) {
	dataDir := filepath.Dir(cfg.Database.Path)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory '%s': %w", dataDir, err)
	}
	db, err := database.NewSQLiteDB(cfg.Database.Path)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite database: %w", err)
	}
	log.Printf("Database initialized successfully at: %s", cfg.Database.Path)
	
	return db, nil
}
