package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Client *mongo.Client
var DB *mongo.Database

// Connect initializes MongoDB connection with optimized pool settings
func Connect(uri string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).
		SetMaxPoolSize(150).           // ⚡ Increased from 80 for high-concurrency uploads
		SetMinPoolSize(30).            // ⚡ Increased from 20 to keep warm connections
		SetMaxConnIdleTime(5*time.Minute).
		SetConnectTimeout(10*time.Second).
		SetSocketTimeout(30*time.Second).
		SetServerSelectionTimeout(10*time.Second).
		SetRetryWrites(true).
		SetRetryReads(true))
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Test connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	Client = client
	DB = client.Database("file-caddy")

	// Create collections and indexes
	if err := createCollectionsAndIndexes(); err != nil {
		return fmt.Errorf("failed to create collections and indexes: %w", err)
	}

	fmt.Println("✅ MongoDB connected successfully (pool: 30-150)")
	return nil
}

// createCollectionsAndIndexes creates necessary collections and indexes
func createCollectionsAndIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collections, err := DB.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return err
	}

	// Create users collection if not exists
	if !containsString(collections, "users") {
		if err := DB.CreateCollection(ctx, "users"); err != nil {
			return err
		}
		fmt.Println("📦 Created users collection")

		// Create unique indexes
		usersCollection := DB.Collection("users")
		emailIndex := mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		}
		usernameIndex := mongo.IndexModel{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		}
		if _, err := usersCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{emailIndex, usernameIndex}); err != nil {
			return err
		}
	}

	// Create files collection if not exists
	if !containsString(collections, "files") {
		if err := DB.CreateCollection(ctx, "files"); err != nil {
			return err
		}
		fmt.Println("📦 Created files collection")

		// Create indexes
		filesCollection := DB.Collection("files")
		indexes := []mongo.IndexModel{
			{Keys: bson.D{{Key: "user_id", Value: 1}}},
			{Keys: bson.D{{Key: "created_at", Value: -1}}},
		}
		if _, err := filesCollection.Indexes().CreateMany(ctx, indexes); err != nil {
			return err
		}
	}

	fmt.Println("✅ MongoDB fully initialized")
	return nil
}

// containsString checks if a slice contains a string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Disconnect closes the MongoDB connection
func Disconnect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := Client.Disconnect(ctx); err != nil {
		return fmt.Errorf("failed to disconnect from MongoDB: %w", err)
	}

	fmt.Println("✅ MongoDB disconnected")
	return nil
}
