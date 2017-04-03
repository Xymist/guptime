package main

import (
	"fmt"
	"log"
	"net/http"
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
	http.HandleFunc("/", homePage)
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
			t := time.Unix(timestamp, 0)
			writer.Write([]byte(fmt.Sprintf("At %v the connection %s", t, status)))
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

func homePage(w http.ResponseWriter, r *http.Request) {
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
