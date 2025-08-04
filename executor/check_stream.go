package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	ctx := context.Background()

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	// Check the existing workflow stream
	streamKey := "events:workflow:simple-test-workflow"
	fmt.Printf("=== Checking stream: %s ===\n", streamKey)
	
	// Read all messages from the stream
	messages, err := redisClient.XRange(ctx, streamKey, "-", "+").Result()
	if err != nil {
		log.Fatal("Failed to read from stream:", err)
	}

	fmt.Printf("Found %d messages in stream\n\n", len(messages))
	
	for i, msg := range messages {
		fmt.Printf("--- Message %d: %s ---\n", i+1, msg.ID)
		
		for key, value := range msg.Values {
			if key == "data" {
				// Try to pretty print JSON data
				if dataStr, ok := value.(string); ok {
					var data map[string]interface{}
					if err := json.Unmarshal([]byte(dataStr), &data); err == nil {
						prettyData, _ := json.MarshalIndent(data, "  ", "  ")
						fmt.Printf("  %s: %s\n", key, string(prettyData))
					} else {
						fmt.Printf("  %s: %s\n", key, dataStr)
					}
				}
			} else {
				fmt.Printf("  %s: %v\n", key, value)
			}
		}
		fmt.Println()
	}

	// Check consumer groups
	fmt.Printf("=== Consumer Groups for %s ===\n", streamKey)
	
	// Try a different approach to get consumer groups
	result, err := redisClient.Do(ctx, "XINFO", "GROUPS", streamKey).Result()
	if err != nil {
		fmt.Printf("Error getting consumer groups: %v\n", err)
	} else {
		fmt.Printf("Consumer groups info: %v\n", result)
	}

	// Check if executor-service consumer group exists
	fmt.Println("\n=== Checking for executor-service consumer group ===")
	consumers, err := redisClient.Do(ctx, "XINFO", "CONSUMERS", streamKey, "executor-service").Result()
	if err != nil {
		fmt.Printf("Error getting consumers for executor-service group: %v\n", err)
		fmt.Println("This suggests the executor service is not running or not subscribed properly")
	} else {
		fmt.Printf("Consumers in executor-service group: %v\n", consumers)
	}
}
