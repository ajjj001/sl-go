package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/fiber/v2/middleware/recover"
	_ "github.com/joho/godotenv/autoload"

	log "github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/go-redis/redis/v8"
)

var cache = redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})

var ctx = context.Background()

type MongoInstance struct {
	Client *mongo.Client
	Db     *mongo.Database
}

var mg MongoInstance

// Database settings (insert your own database name and connection URI)
const dbName = "ror_api"

func Connect() error {
	client, err := mongo.NewClient(options.Client().ApplyURI(os.Getenv("URI")))
	if err != nil {
		log.Fatal(err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db := client.Database(dbName)
	err = client.Connect(ctx)

	if err != nil {
		log.Fatal(err)
		return err
	}

	mg = MongoInstance{
		Client: client,
		Db:     db,
	}

	return nil
}

type User struct {
	ID        string `bson:"_id,omitempty"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Gender    string `json:"gender"`
	Age       int    `json:"age"`
}

func (user User) Validate() error {
	return validation.ValidateStruct(&user,
		validation.Field(&user.FirstName, validation.Required),
		validation.Field(&user.LastName, validation.Required),
		validation.Field(&user.Age, validation.Min(0)),
		validation.Field(&user.Gender, validation.In("male", "female", "others")))
}

func verifyCache(c *fiber.Ctx) error {
	id := c.Params("id")
	val, err := cache.Get(ctx, id).Bytes()
	if err != nil {
		return c.Next()
	}

	user := User{}
	err = json.Unmarshal(val, &user)
	if err != nil {
		c.Next()
	}

	return c.JSON(fiber.Map{"Cached": user})

}

func SetupRoutes(app *fiber.App) {
	// Panic recovery middleware
	app.Use(recover.New())

	app.Get("/users/count", func(c *fiber.Ctx) error {
		count, err := mg.Db.Collection("users").CountDocuments(c.Context(), bson.M{})
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{
			"count": count,
		})
	})

	// CRUD for user
	// Create
	app.Post("/users", func(c *fiber.Ctx) error {
		var user User

		if err := c.BodyParser(&user); err != nil {
			c.Status(400).SendString("Error parsing body")
		}

		if err := user.Validate(); err != nil {
			c.Status(400).SendString("Invalid user data")
			return err
		}

		res, err := mg.Db.Collection("users").InsertOne(c.Context(), user)
		if err != nil {
			return c.Status(500).SendString("Error inserting user")
		}

		return c.Status(201).JSON(res)

	})

	// Read
	app.Get("/users/:id", verifyCache, func(c *fiber.Ctx) error {

		params := c.Params("id")
		id, err := primitive.ObjectIDFromHex(params)

		if err != nil {
			return c.Status(500).SendString("Error parsing id")
		}

		var user User
		filter := bson.M{"_id": id}
		if err := mg.Db.Collection("users").FindOne(c.Context(), filter).Decode(&user); err != nil {
			return c.Status(500).SendString("Error finding user")
		}

		// unmarshal user to json
		val, err := json.Marshal(user)
		if err != nil {
			panic(err)
		}

		cacheErr := cache.Set(ctx, params, val, 30*time.Second).Err()
		if cacheErr != nil {
			return cacheErr
		}

		return c.JSON(user)

	})

	// Update
	app.Put("/users/:id", func(c *fiber.Ctx) error {

		params := c.Params("id")
		id, err := primitive.ObjectIDFromHex(params)

		if err != nil {
			return c.Status(500).SendString("Error parsing id")
		}

		var user User
		if err := c.BodyParser(&user); err != nil {
			return c.Status(400).SendString("Error parsing body")
		}

		filter := bson.M{"_id": id}
		update := bson.M{"$set": user}
		res, err := mg.Db.Collection("users").UpdateOne(c.Context(), filter, update)
		if err != nil {
			return c.Status(500).SendString("Error updating user")
		}

		return c.JSON(res)

	})

	// Delete
	app.Delete("/users/:id", func(c *fiber.Ctx) error {

		params := c.Params("id")
		id, err := primitive.ObjectIDFromHex(params)

		if err != nil {
			return c.Status(500).SendString("Error parsing id")
		}

		filter := bson.M{"_id": id}
		res, err := mg.Db.Collection("users").DeleteOne(c.Context(), filter)
		if err != nil {
			return c.Status(500).SendString("Error deleting user")
		}

		return c.JSON(res)

	})

	// This route is protected by Basic Auth
	app.Get("/",
		basicauth.New(basicauth.Config{
			Users: map[string]string{
				"john":  "doe",
				"admin": "123456",
			},
		}),
		func(c *fiber.Ctx) error {
			log.Info("Received / request")
			return c.SendString("Hello, World 👋!")
		})

	app.Get("/healthcheck", func(c *fiber.Ctx) error {
		log.Info("Received /healthcheck request")
		return c.SendString("OK")
	})
}

func main() {

	// Sample validation
	u1 := User{
		FirstName: "sam",
		LastName:  "chan",
		Gender:    "male",
		Age:       20,
	}

	if err := u1.Validate(); err != nil {
		fmt.Println(err)
	}

	log.SetFormatter(&log.JSONFormatter{})

	log.WithFields(log.Fields{
		"user": "admin",
	}).Info("Starting server...")

	if err := Connect(); err != nil {
		log.Fatal(err)
	}

	app := fiber.New()

	SetupRoutes(app)

	// Timeout middleware
	// handler := func(c *fiber.Ctx) error {
	// 	err := c.SendString("Timeout!")
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return nil
	// }

	// app.Use(timeout.New(handler, 5*time.Second))

	app.Listen(fmt.Sprintf(":%s", os.Getenv("PORT")))
}
