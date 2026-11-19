package main

import (
	"context"
	"file-sharing-server/database"
	"file-sharing-server/middleware"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)


func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using environment variables")
	}

	ctx := context.Background()

	// MongoDB connection
	mongoClient, err := database.ConnectMongo(
		ctx,
		os.Getenv("MONGODB_URI"),
	)
	if err != nil {
		log.Fatal("MongoDB connection failed:", err)
	}
	defer mongoClient.Disconnect(context.Background())

	log.Println("MongoDB connected successfully")

	// Redis configuration
	useTLS, err := strconv.ParseBool(os.Getenv("REDIS_TLS"))
	if err != nil {
		log.Fatal("Invalid REDIS_TLS value:", err)
	}

	redisClient, err := database.ConnectRedis(
		ctx,
		os.Getenv("REDIS_ADDR"),
		os.Getenv("REDIS_USERNAME"),
		os.Getenv("REDIS_PASSWORD"),
		useTLS,
	)
	if err != nil {
		log.Fatal("Redis connection failed:", err)
	}
	defer redisClient.Close()

	log.Println("Redis connected successfully")


	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    ":8080",
		Handler: middleware.LoggingMiddleware(mux),
	}
	log.Println("Server running on port 8080")
	log.Fatal(server.ListenAndServe())
}
