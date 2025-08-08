package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	domainUseCase "mailsafe/domain/domain"
	"mailsafe/domain/email"
	"mailsafe/internal/repository/pg"
	"mailsafe/internal/smtp"

	"github.com/guilhermebr/gox/logger"
	"github.com/guilhermebr/gox/postgres"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	// Initialize logger
	logger, err := logger.NewLogger("")
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	// Initialize database connection
	db, err := postgres.New(context.Background(), "")
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initialize repositories
	repo := pg.NewRepository(db)

	// Initialize use cases
	domainUseCase := domainUseCase.NewUseCase(repo.DomainRepo)
	emailUseCase := email.NewUseCase(repo.EmailAddressRepo, repo.ReceivedEmailRepo, repo.DomainRepo)

	// Create SMTP server
	smtpServer, err := smtp.NewServer(domainUseCase, emailUseCase, logger)
	if err != nil {
		logger.Error("Failed to create SMTP server", "error", err)
		os.Exit(1)
	}

	// Start server in goroutine
	go func() {
		if err := smtpServer.Start(); err != nil {
			logger.Error("SMTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down SMTP server...")

	// Stop server gracefully
	ctx := context.Background()
	if err := smtpServer.Stop(ctx); err != nil {
		logger.Error("Error during SMTP server shutdown", "error", err)
	}

	logger.Info("SMTP server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
