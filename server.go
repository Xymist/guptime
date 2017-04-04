package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func server(db *bolt.DB, nc chan int64) {
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		serveWS(db, nc, w, r)
	})
	http.HandleFunc("/assets/", assetHandler)
	http.HandleFunc("/", mainPage)
	log.Fatal(http.ListenAndServe("127.0.0.1:9000", nil))
}

// Websockets because A: neat, and B: the data change here is hopefully infrequent,
// so polling would be extremely wasteful; we're better off pushing it as it arrives.
func serveWS(db *bolt.DB, nc chan int64, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	for {
		select {
		case timestamp := <-nc:
			writer, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			status := getStatus(db, timestamp)
			// This probably could be refactored; boolean -> []byte -> string -> string
			// is a bit of a roundabout set of coercions to go through.
			if status == "true" {
				status = "came up."
			}
			if status == "false" {
				status = "went down."
			}
			time := time.Unix(timestamp, 0)
			writer.Write([]byte(fmt.Sprintf("At %v the connection %s", time, status)))
			if err := writer.Close(); err != nil {
				return
			}
		}
	}
}

// Wrapper for getting the status for a given timestamp
func getStatus(db *bolt.DB, timestamp int64) (value string) {
	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("stateChanges"))
		value = string(bkt.Get([]byte(fmt.Sprint(timestamp))))
		return nil
	})
	return
}

func mainPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	homepage, err := Asset("home.html")
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}
	w.Write(homepage)
}

func assetHandler(w http.ResponseWriter, r *http.Request) {
	// Strip the first / off to get the actual path
	path := r.URL.Path[len("/"):]

	// Retrieve the asset from storage
	asset, err := Asset(path)
	if err != nil {
		http.Error(w, "Not found", 404)
		return
	}

	// Set the content type header
	var contentType string
	if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(path, ".png") {
		contentType = "image/png"
	} else {
		contentType = "text/plain"
	}
	w.Header().Add("Content-Type", contentType)

	// Respond
	w.Write(asset)
}
