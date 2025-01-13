package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/thek4n/paste.thek4n.name/cmd/storage"
)

type Users struct {
	db storage.KeysDB
}

func main() {
	log.Println("Connecting to database...")

	db, err := storage.InitStorageDB()
	if err != nil {
		log.Fatalf("failed to connect to database server: %s\n", err.Error())
		return
	}

	users := Users{db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{key}", users.getHandler)
	mux.HandleFunc("POST /", users.saveHandler)

	log.Print("Server started on 0.0.0.0:80 ...")

	log.Fatal(http.ListenAndServe("0.0.0.0:80", mux))
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

	uniqKey, err := generateUniqKey(users.db)
	if err != nil {
		log.Printf("Error on generating unique key: %s, suffered user %s", err.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = users.db.Set(uniqKey, body)
	if err != nil {
		log.Printf(
			"Error on setting key: %s",
			err.Error(),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Set content with size %d on key '%s' from %s", len(body), uniqKey, r.RemoteAddr)

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

	content, err := users.db.Get(key)

	if content == nil {
		w.WriteHeader(http.StatusNotFound)

		_, err := w.Write([]byte("404 Not Found"))
		if err != nil {
			log.Printf("Error on answer: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Printf("Not found by key '%s' from %s", key, r.RemoteAddr)
		return
	}

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
	log.Printf("Get content by key '%s' from %s", key, r.RemoteAddr)
}

func detectScheme(r *http.Request) string {
	if r.TLS == nil {
		return "http://"
	} else {
		return "https://"
	}
}

func generateUniqKey(db storage.KeysDB) (string, error) {
	length := 14

	key := generateKey(length)

	exists, err := db.Exists(key)
	if err != nil {
		return "", err
	}

	if exists {
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
