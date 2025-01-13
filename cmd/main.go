package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thek4n/paste.thek4n.name/cmd/storage"
)

type Users struct {
	Db *redis.Client
}

func main() {
	redisHost := os.Getenv("REDIS_HOST")

	cfg := storage.Config{
		Addr:        redisHost + ":6379",
		Password:    "",
		User:        "",
		DB:          0,
		MaxRetries:  5,
		DialTimeout: 10 * time.Second,
		Timeout:     5 * time.Second,
	}

	log.Printf("Connecting to redis via %s:6379 ...", redisHost)
	db, err := storage.NewClient(context.Background(), cfg)
	if err != nil {
		panic(err)
	}

	users := Users{Db: db}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{key}", users.getHandler)
	mux.HandleFunc("POST /", users.saveHandler)

	log.Print("Server started on 0.0.0.0:80 ...")

	panic(http.ListenAndServe("0.0.0.0:80", mux))
}

func (users *Users) saveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf(
			"Error on reading body: %s. Response to client with code %d",
			err.Error(),
			http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	uniqKey, err := generateUniqKey(users.Db)
	if err != nil {
		log.Printf("Error on generating unique key: %s, suffered user %s", err.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = users.Db.Set(context.Background(), uniqKey, body, 0).Err()
	if err != nil {
		log.Printf(
			"Error on setting key: %s",
			err.Error(),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Set body with key '%s'", uniqKey)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	scheme := detectScheme(r)

	_, err = fmt.Fprintf(w, "%s%s/%s", scheme, r.Host, uniqKey)
	if err != nil {
		log.Printf("Error on answer: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (users *Users) getHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	key := r.PathValue("key")

	result := users.Db.Get(context.Background(), key)

	if result.Err() == redis.Nil {
		w.WriteHeader(http.StatusNotFound)

		_, err := w.Write([]byte("404 Not Found"))
		if err != nil {
			log.Printf("Error on answer: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		return
	}

	if result.Err() != nil {
		log.Printf(
			"Error on getting key: %s",
			result.Err().Error(),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	content, err := result.Bytes()
	if err != nil {
		log.Printf(
			"Error on getting key: %s",
			err.Error(),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(content)
	if err != nil {
		log.Printf("Error on answer: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func detectScheme(r *http.Request) string {
	if r.TLS == nil {
		return "http://"
	} else {
		return "https://"
	}
}

func generateUniqKey(db *redis.Client) (string, error) {
	length := 14

	key := generateKey(length)

	keysNumber, err := db.Exists(context.Background(), key).Uint64()
	if err != nil {
		return "", err
	}

	if keysNumber > 0 {
		return generateUniqKey(db)
	}

	return key, nil
}

func generateKey(length int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[r.Intn(len(chars))]
	}

	return string(result)
}
