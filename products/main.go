package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func IndexProductWithRetry(client *elasticsearch.Client, product Product) error {
	// Serialize the product to JSON
	productJSON, err := json.Marshal(product)
	if err != nil {
		return err
	}

	// Prepare the index request
	req := esapi.IndexRequest{
		Index:      "products",
		DocumentID: product.ID,
		Body:       strings.NewReader(string(productJSON)),
	}

	// Retry with backoff
	err = retryWithBackoff(3, func() error {
		// Execute the request
		res, err := req.Do(context.Background(), client)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		// Check for errors in the response
		if res.IsError() {
			body, _ := io.ReadAll(res.Body)
			return fmt.Errorf("error indexing product: %s - Status Code: %d - Body: %s", res.String(), res.StatusCode, string(body))
		}

		return nil
	})

	return err
}

// Retry function with exponential backoff
func retryWithBackoff(maxRetries int, fn func() error) error {
	for i := 0; i < maxRetries; i++ {
		if err := fn(); err == nil {
			return nil
		}
		time.Sleep(time.Duration(1<<uint(i)*100) * time.Millisecond) // Increase the backoff time
	}
	return fmt.Errorf("max retries exceeded")
}

type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
}

func main() {
	// Create an Elasticsearch client
	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Second * 30, // Set a longer timeout here
		},
	}
	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating Elasticsearch client: %s", err)
	}

	log.Println("Successfully connected to Elasticsearch")

	// Example product
	for i := 1; i <= 1000; i++ {
		product := Product{
			ID:          fmt.Sprintf("%d", i),
			Name:        fmt.Sprintf("Product %d", i),
			Description: fmt.Sprintf("Description for Product %d", i),
			Category:    "Electronics",   // You can customize the category
			Price:       float64(i * 10), // You can customize the pricing logic
		}

		// Index the product with retry
		err := IndexProductWithRetry(esClient, product)
		if err != nil {
			log.Printf("Error indexing product %d: %s", i, err.Error())
		}
	}

	log.Println("Product indexed successfully!")
}
