package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type DataForDb struct {
	Source string `bson:"source"`
	Key    string `bson:"key"`
	Url    string `bson:"url"`
	Desc   string `bson:"desc"`
}

func ConnectToDb() *mongo.Client {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("You must set your 'MONGODB_URI' environment variable")
	}

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)

	client, err := mongo.Connect(opts)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Database("admin").
		RunCommand(ctx, bson.D{{"ping", 1}}).
		Err()
	if err != nil {
		panic(err)
	}

	fmt.Println("Connected to MongoDB")

	ensureIndexes(client)

	return client
}

func ensureIndexes(client *mongo.Client) {
	collection := client.
		Database("darkwebScrapingData").
		Collection("linksAndDesc")

	index := mongo.IndexModel{
		Keys: bson.D{
			{Key: "source", Value: 1},
			{Key: "url", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := collection.Indexes().CreateOne(context.TODO(), index)
	if err != nil {
		panic(err)
	}
}

func BatchInsert(client *mongo.Client, batch []DataForDb) {
	collection := client.
		Database("darkwebScrapingData").
		Collection("linksAndDesc")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.InsertMany(ctx, batch)
	if err != nil {
		// handle duplicate key errors (code 11000)
		if we, ok := err.(mongo.BulkWriteException); ok {
			for _, e := range we.WriteErrors {
				if e.Code != 11000 {
					log.Printf("non-duplicate error: %v", e)
				}
			}
			return
		}
		panic(err)
	}
}

func AddDataToDb(client *mongo.Client, ch <-chan DataForDb) {
	batch := make([]DataForDb, 0, 50)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	// fmt.Printf("[Receiver] ch addr: %p\n", chs)
	for {
		select {
		case data, ok := <-ch:
			fmt.Println(data)
			if !ok {
				if len(batch) > 0 {
					BatchInsert(client, batch)
				}
				return
			}

			batch = append(batch, data)
			fmt.Println(batch)
			if len(batch) >= 50 {
				BatchInsert(client, batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// fmt.Println(batch)

			if len(batch) > 0 {
				BatchInsert(client, batch)
				batch = batch[:0]
			}
		}
	}
}
