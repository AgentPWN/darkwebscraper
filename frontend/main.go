package main

import (
	"context"
	"darkwebscraper/utils"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var client *mongo.Client

var allKeys []string

type QueryRequest struct {
	Database   string   `json:"database"`
	Collection string   `json:"collection"`
	Keys       []string `json:"keys"`
}

type QueryResponse struct {
	Results []map[string]interface{} `json:"results"`
	Count   int                      `json:"count"`
	Error   string                   `json:"error,omitempty"`
}

type DatabasesResponse struct {
	Databases []string `json:"databases"`
	Error     string   `json:"error,omitempty"`
}

type CollectionsResponse struct {
	Collections []string `json:"collections"`
	Error       string   `json:"error,omitempty"`
}

func loadPage(filename string) []byte {
	html, _ := os.ReadFile("frontend/" + filename + ".html")
	return html
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Write(loadPage("index"))
	}
}
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func keysHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allKeys)
}

func databasesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbs, err := client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		json.NewEncoder(w).Encode(DatabasesResponse{Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(DatabasesResponse{Databases: dbs})
}

func collectionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dbName := r.URL.Query().Get("db")
	if dbName == "" {
		json.NewEncoder(w).Encode(CollectionsResponse{Error: "db parameter required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cols, err := client.Database(dbName).ListCollectionNames(ctx, bson.M{})
	if err != nil {
		json.NewEncoder(w).Encode(CollectionsResponse{Error: err.Error()})
		return
	}
	json.NewEncoder(w).Encode(CollectionsResponse{Collections: cols})
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(QueryResponse{Error: "Invalid request body"})
		return
	}

	if len(req.Keys) == 0 {
		json.NewEncoder(w).Encode(QueryResponse{Error: "At least one key must be selected"})
		return
	}

	// if req.Database == "" || req.Collection == "" {
	// 	json.NewEncoder(w).Encode(QueryResponse{Error: "Database and collection are required"})
	// 	return
	// }
	filter := bson.M{"key": bson.M{"$in": req.Keys}}
	projection := bson.D{}
	for _, key := range req.Keys {
		projection = append(projection, bson.E{Key: key, Value: 1})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// fmt.Println(req.Database, req.Collection)
	// fmt.Println(projection)
	// col := client.Database(req.Database).Collection(req.Collection)
	// opts := options.Find().SetProjection(projection).SetLimit(500)
	col := client.Database("darkwebScrapingData").Collection("linksAndDesc")

	opts := options.Find().SetProjection(bson.M{
		"source": 1,
		"url":    1,
		"desc":   1,
		"key":    1,
		"_id":    0,
	})
	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}
	// fmt.Println(results)
	if results == nil {
		results = []map[string]interface{}{}
	}

	json.NewEncoder(w).Encode(QueryResponse{
		Results: results,
		Count:   len(results),
	})
}

func main() {
	client = utils.ConnectToDb()
	contents, _ := os.ReadFile("../names.txt")
	allKeys = strings.Split(string(contents), "\n")
	for i := range len(allKeys) {
		allKeys[i] = strings.TrimSpace(allKeys[i])
	}

	// For checking is db is up
	// var result bson.M
	// err := client.
	// 	Database("darkwebScrapingData").
	// 	Collection("linksAndDesc").
	// 	FindOne(context.TODO(), bson.M{}).
	// 	Decode(&result)

	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Printf("%+v\n", result)
	http.Handle("/", corsMiddleware(Handler))
	http.HandleFunc("/api/keys", corsMiddleware(keysHandler))
	http.HandleFunc("/api/databases", corsMiddleware(databasesHandler))
	http.HandleFunc("/api/collections", corsMiddleware(collectionsHandler))
	http.HandleFunc("/api/query", corsMiddleware(queryHandler))

	log.Println("Server running on http://localhost:", 8080)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
