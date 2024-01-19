package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func SearchHandler(client *elasticsearch.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query parameters
		query := r.URL.Query().Get("query")
		category := r.URL.Query().Get("category")
		priceRange := r.URL.Query().Get("priceRange")

		// Build the Elasticsearch query
		var queryDSL string
		if query != "" {
			queryDSL += fmt.Sprintf(`{"query":{"match":{"name":"%s"}}}`, query)
		}

		if category != "" {
			if queryDSL != "" {
				queryDSL += ","
			}
			queryDSL += fmt.Sprintf(`{"filter":{"term":{"category":"%s"}}}`, category)
		}

		if priceRange != "" {
			// Parse priceRange into min and max values
			var minPrice, maxPrice float64
			_, err := fmt.Sscanf(priceRange, "%f-%f", &minPrice, &maxPrice)
			if err != nil {
				http.Error(w, "Invalid price range format", http.StatusBadRequest)
				return
			}

			// Add price range filter to the query
			if queryDSL != "" {
				queryDSL += ","
			}
			queryDSL += fmt.Sprintf(`{"filter":{"range":{"price":{"gte":%f,"lte":%f}}}}`, minPrice, maxPrice)
		}

		// Combine query parts
		if queryDSL != "" {
			queryDSL = fmt.Sprintf(`{"bool":{%s}}`, queryDSL)
		}

		// Prepare the search request
		req := esapi.SearchRequest{
			Index: []string{"products"},
			Body:  strings.NewReader(queryDSL),
		}

		// Execute the request
		res, err := req.Do(context.Background(), client)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error executing search: %s", err), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()

		// Check for errors in the response
		if res.IsError() {
			http.Error(w, fmt.Sprintf("Error in search response: %s", res.Status()), http.StatusInternalServerError)
			return
		}

		// Print the response status for debugging
		fmt.Printf("Search response status: %s\n", res.Status())

		// Decode the response body
		var result map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			http.Error(w, fmt.Sprintf("Error decoding search response: %s", err), http.StatusInternalServerError)
			return
		}

		// Send the search result as JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func main() {
	router := http.NewServeMux()

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

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World!")
	})

	router.HandleFunc("/search", SearchHandler(esClient))

	// Start the server
	log.Println("Starting the server...")

	err = http.ListenAndServe(":8080", router)
	if err != nil {
		log.Fatalf("Error starting the server: %s", err)
	}
}
