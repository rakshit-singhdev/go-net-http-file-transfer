package main

import (
	"context"
	"errors"
	"file-sharing-server/database"
	"file-sharing-server/handlers"
	"file-sharing-server/helpers"
	"file-sharing-server/middleware"
	"file-sharing-server/repositories"
	"file-sharing-server/services"
	"file-sharing-server/storage"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func run() error {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using environment variables")
	}

	// Root application context
	ctx := context.Background()

	// MongoDB connection
	mongoClient, err := database.ConnectMongo(
		ctx,
		os.Getenv("MONGODB_URI"),
	)
	if err != nil {
		return err
	}

	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Println("MongoDB disconnect error:", err)
		}
	}()

	log.Println("MongoDB connected successfully")

	// Redis configuration
	useTLS, err := strconv.ParseBool(os.Getenv("REDIS_TLS"))
	if err != nil {
		return err
	}

	// Redis connection
	redisClient, err := database.ConnectRedis(
		ctx,
		os.Getenv("REDIS_ADDR"),
		os.Getenv("REDIS_USERNAME"),
		os.Getenv("REDIS_PASSWORD"),
		useTLS,
	)
	if err != nil {
		return err
	}

	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Println("Redis disconnect error:", err)
		}
	}()

	log.Println("Redis connected successfully")

	// MongoDB repository
	mongoDB := mongoClient.Database(os.Getenv("MONGODB_DATABASE"))

	fileRepository := repositories.NewFileRepository(mongoDB)

	// Initialize MongoDB indexes
	indexCtx, indexCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	if err := fileRepository.EnsureIndexes(indexCtx); err != nil {
		indexCancel()
		return err
	}

	indexCancel()

	log.Println("MongoDB indexes initialized")

	// Local file storage
	localStorage, err := storage.NewLocalStorage("uploads")
	if err != nil {
		return err
	}

	log.Println("Local file storage initialized")

	// File service
	fileService := services.NewFileService(
		fileRepository,
		localStorage,
		24*time.Hour,
	)

	log.Println("File service initialized")

	// HTTP routes
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("File Sharing Server is running"))
	})

	fileHandler := handlers.NewFileHandler(
		fileService,
		helpers.ParseUploadLimit(os.Getenv("MAX_UPLOAD_SIZE"), 2<<20),
	)

	mux.HandleFunc("POST /api/files", fileHandler.Upload)
	mux.HandleFunc("GET /api/files/{id}", fileHandler.GetMetadata)

	// HTTP server configuration
	server := &http.Server{
		Addr:         ":" + getPort(),
		Handler:      middleware.LoggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Register shutdown signals
	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Start HTTP server
	serverErrors := make(chan error, 1)

	go func() {
		log.Println("Local: http://localhost" + server.Addr)
		for _, ip := range helpers.GetLocalIPs() {
			log.Println("IPv4:  http://" + ip + server.Addr)
		}

		serverErrors <- server.ListenAndServe()
	}()

	// Wait for server error or shutdown signal
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}

	case <-signalCtx.Done():
		log.Println("Shutdown signal received")
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Println("Server stopped gracefully")

	return nil
}

func getPort() string {
	port := os.Getenv("PORT")

	if port == "" {
		return "8080"
	}

	return port
}

func main() {
	if err := run(); err != nil {
		log.Println("Application error:", err)
		os.Exit(1)
	}
}
