package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/thek4n/paste.thek4n.name/internal/keys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

type Handlers struct {
	Db storage.KeysDB
}

func (handlers *Handlers) Pingpong(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err := fmt.Fprint(w, "pong")

	if err != nil {
		log.Printf("Error on answer ping: %s, suffered user %s", err.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (handlers *Handlers) Cache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf(
			"Error on reading body: %s. Response to client %s with code %d",
			err.Error(),
			r.RemoteAddr,
			http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	key, err := keys.Cache(handlers.Db, body)

	if err != nil {
		log.Printf("Error on setting key: %s, suffered user %s", err.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("Set content with size %d on key '%s' from %s", len(body), key, r.RemoteAddr)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	scheme := detectScheme(r)

	_, err = fmt.Fprintf(w, "%s%s/%s/", scheme, r.Host, key)
	if err != nil {
		log.Printf("Error on answer: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (handlers *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	key := r.PathValue("key")

	content, err := keys.Get(handlers.Db, key)

	if err == storage.ErrKeyNotFound || errors.Unwrap(err) == storage.ErrKeyNotFound {
		w.WriteHeader(http.StatusNotFound)

		_, err := w.Write([]byte("404 Not Found"))
		if err != nil {
			log.Printf("Error on answer: %s, suffered user %s", err.Error(), r.RemoteAddr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Printf("Not found by key '%s' from %s", key, r.RemoteAddr)
		return
	}

	if err != nil {
		log.Printf(
			"Error on getting key: %s, suffered user %s",
			err.Error(),
			r.RemoteAddr,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(content)
	if err != nil {
		log.Printf("Error on answer: %s, suffered user %s", err.Error(), r.RemoteAddr)
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
