package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/thek4n/paste.thek4n.name/internal/keys"
	"github.com/thek4n/paste.thek4n.name/internal/storage"
)

const ONE_MEBIBYTE = 1048576

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

	if r.ContentLength > ONE_MEBIBYTE {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	body := make([]byte, r.ContentLength)

	_, readBodyErr := r.Body.Read(body)

	if readBodyErr != io.EOF {
		log.Printf(
			"Error on reading body: %s. Response to client %s with code %d",
			readBodyErr.Error(),
			r.RemoteAddr,
			http.StatusInternalServerError,
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	key, cacheErr := keys.Cache(handlers.Db, body, 4*time.Second)

	if cacheErr != nil {
		log.Printf("Error on setting key: %s, suffered user %s", cacheErr.Error(), r.RemoteAddr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("Set content with size %d on key '%s' from %s", len(body), key, r.RemoteAddr)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	scheme := detectScheme(r)

	_, answerErr := fmt.Fprintf(w, "%s%s/%s/", scheme, r.Host, key)
	if answerErr != nil {
		log.Printf("Error on answer: %s", answerErr.Error())
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

	content, getKeyErr := keys.Get(handlers.Db, key, 4*time.Second)

	if getKeyErr != nil {
		if getKeyErr == storage.ErrKeyNotFound || errors.Unwrap(getKeyErr) == storage.ErrKeyNotFound {
			w.WriteHeader(http.StatusNotFound)

			_, writeErr := w.Write([]byte("404 Not Found"))
			if writeErr != nil {
				log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			log.Printf("Not found by key '%s' from %s", key, r.RemoteAddr)
			return
		} else {
			log.Printf(
				"Error on getting key: %s, suffered user %s",
				getKeyErr.Error(),
				r.RemoteAddr,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(content)
	if writeErr != nil {
		log.Printf("Error on answer: %s, suffered user %s", writeErr.Error(), r.RemoteAddr)
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
