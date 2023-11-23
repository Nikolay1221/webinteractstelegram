package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	database := client.Database("PhoneModels")
	fsFiles := database.Collection("fs.files")
	bucket, err := gridfs.NewBucket(database)
	if err != nil {
		log.Fatal(err)
	}

	cursor, err := fsFiles.Find(ctx, bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var result bson.M
		if err = cursor.Decode(&result); err != nil {
			log.Fatal(err)
		}

		fileID, ok := result["_id"].(primitive.ObjectID)
		if !ok {
			log.Fatal("Cannot convert to ObjectID")
		}

		filename, ok := result["filename"].(string)
		if !ok {
			log.Fatal("Cannot convert filename to string")
		}

		uniqueFilename := fmt.Sprintf("%s_%s.jpg", filename, fileID.Hex())
		buf := bytes.NewBuffer(nil)

		_, err = bucket.DownloadToStream(fileID, buf)
		if err != nil {
			log.Fatal(err)
		}

		err = ioutil.WriteFile(uniqueFilename, buf.Bytes(), 0644)
		if err != nil {
			log.Fatal(err)
		}

		sendPhotoToTelegram(uniqueFilename)

		err = bucket.Delete(fileID)
		if err != nil {
			log.Fatal(err)
		}

		err = os.Remove(uniqueFilename)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}
}

func sendPhotoToTelegram(filename string) {
	bot, err := tgbotapi.NewBotAPI("6588492712:AAHr0tK6WvmrAgHYWYq1oSm4sZ5-yRdX_GU")
	if err != nil {
		log.Fatal(err)
	}

	chatID := int64(1231104328)

	brand, webID, phoneNumber, description, complications := parseFilename(filename)

	messageText := fmt.Sprintf("Марка: %s\nWeb ID: %s\nНомер телефона: %s\nОписание: %s\nКомплектация: %s",
		brand, webID, phoneNumber, description, complications)

	textMsg := tgbotapi.NewMessage(chatID, messageText)
	_, err = bot.Send(textMsg)
	if err != nil {
		log.Fatal(err)
	}

	photoBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	photoFileBytes := tgbotapi.FileBytes{
		Name:  filename,
		Bytes: photoBytes,
	}
	photoMsg := tgbotapi.NewPhotoUpload(chatID, photoFileBytes)
	_, err = bot.Send(photoMsg)
	if err != nil {
		log.Fatal(err)
	}
}

func parseFilename(filename string) (string, string, string, string, string) {
	parts := strings.Split(filename, "_")
	brand := parts[0]

	details := strings.Split(parts[1], "?")
	webID := details[0]
	phoneNumber := details[1]
	description := details[2]
	complications := details[3]

	return brand, webID, phoneNumber, description, complications
}
