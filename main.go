package main

import (
	"context"
	"errors"
	"file-sharing-server/cache"
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

	_ "file-sharing-server/docs"

	"github.com/joho/godotenv"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

//	@title			File Sharing Server API
//	@version		1.0
//	@description	Upload, inspect and download temporary files.
//	@BasePath		/
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Enter "Bearer" followed by a space and your JWT.

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
	userRepository := repositories.NewUserRepository(mongoDB)

	// Initialize MongoDB indexes
	indexCtx, indexCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	if err := fileRepository.EnsureIndexes(indexCtx); err != nil {
		indexCancel()
		return err
	}

	if err := userRepository.EnsureIndexes(indexCtx); err != nil {
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

	// JWT configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		return errors.New("JWT_SECRET is not set or too short")
	}

	tokenDuration, err := time.ParseDuration(os.Getenv("JWT_EXPIRES_IN"))
	if err != nil {
		return err
	}

	log.Println("JWT configuration initialized with duration", tokenDuration)

	// File service
	// fileTTL := helpers.ParseDuration(os.Getenv("FILE_TTL"), 24*time.Hour)
	fileTTL := helpers.ParseDuration(os.Getenv("FILE_TTL"), 30*time.Second)

	fileService := services.NewFileService(
		fileRepository,
		localStorage,
		cache.NewRedisCache(redisClient),
		fileTTL,
	)
	authService := services.NewAuthService(
		userRepository,
		jwtSecret,
		tokenDuration,
	)

	log.Println("File service initialized with TTL", fileTTL)

	// Apply the current TTL to files uploaded before it was changed
	ttlCtx, ttlCancel := context.WithTimeout(
		context.Background(),
		// 24*time.Hour,
		30*time.Second,
	)

	updated, err := fileService.ApplyTTLToAllFiles(ttlCtx)
	ttlCancel()

	if err != nil {
		return err
	}

	log.Printf("File TTL applied; %d existing files updated", updated)

	// Background cleanup of expired files
	cleanupCtx, cleanupCancel := context.WithCancel(
		context.Background(),
	)
	defer cleanupCancel()

	go services.RunCleanupWorker(
		cleanupCtx,
		fileService,
		time.Minute,
	)

	log.Println("Cleanup worker started")

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
	authHandler := handlers.NewAuthHandler(authService)

	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)

	mux.Handle("POST /api/files", middleware.RateLimitMiddleware(redisClient, 10, time.Minute)(middleware.AuthMiddleware(jwtSecret)(
		http.HandlerFunc(fileHandler.Upload),
	)))
	mux.HandleFunc("GET /api/files/{id}", fileHandler.GetMetadata)
	mux.HandleFunc("GET /api/files/", fileHandler.List)
	mux.HandleFunc("GET /api/files/{id}/download", fileHandler.Download)
	mux.HandleFunc("DELETE /api/files/{id}", fileHandler.Delete)

	// Swagger UI: http://localhost:<port>/swagger/index.html
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

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
