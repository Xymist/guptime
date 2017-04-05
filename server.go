package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	http.HandleFunc("/", homePage)
	http.HandleFunc("/assets/", assetHandler)
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
	// Commented out because it currently causes serious issues with the charts.
	//initHistory(db, conn)
	for {
		select {
		// Regardless of state change, we should push every five minutes for visibility.
		case <-time.After(300 * time.Second):
			pushData(db, conn, time.Now().Unix())
		case timestamp := <-nc:
			pushData(db, conn, timestamp)
		}
	}
}

func pushData(db *bolt.DB, conn *websocket.Conn, timestamp int64) {
	writer, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return
	}

	status := getStatus(db, timestamp)
	if status == "" {
		timestamp, status = getLatest(db)
	}

	// This probably could be refactored; boolean -> []byte -> string -> string
	// is a bit of a roundabout set of coercions to go through.
	if status == "true" {
		status = "came up."
	}
	if status == "false" {
		status = "went down."
	}
	time := time.Unix(timestamp, 0).UnixNano() / 1000000

	// Really, I could just have JS do the string and only send it the timestamp/status
	writer.Write([]byte(fmt.Sprintf("At %v the connection %s", time, status)))
	if err := writer.Close(); err != nil {
		return
	}
}

func initHistory(db *bolt.DB, conn *websocket.Conn) {
	var message string
	_, msg, err := conn.ReadMessage()
	if err != nil {
		fmt.Println("Receive Failed: " + err.Error())
	}
	if msg != nil {
		message = string(msg)
	}
	if strings.Contains(message, "init") {
		writer, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}
		times, statuses := readHistory(db)
		hs := [][]string{times, statuses}
		history, err := json.Marshal(hs)
		if err != nil {
			return
		}
		writer.Write(history)
		if err := writer.Close(); err != nil {
			return
		}
	}
}

func readHistory(db *bolt.DB) (times []string, statuses []string) {
	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("stateChanges"))
		bkt.ForEach(func(k []byte, v []byte) error {
			times = append(times, string(k))
			statuses = append(statuses, string(v))
			return nil
		})
		return nil
	})
	return
}

// Wrapper for getting the status for a given timestamp
func getStatus(db *bolt.DB, timestamp int64) (status string) {
	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("stateChanges"))
		if value := bkt.Get([]byte(fmt.Sprint(timestamp))); value != nil {
			status = string(value)
			return nil
		}
		status = ""
		return nil
	})
	return
}

func getLatest(db *bolt.DB) (timestamp int64, status string) {
	db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("stateChanges")).Cursor()
		k, v := c.Last()
		i, _ := strconv.Atoi(string(k))
		timestamp = int64(i)
		status = string(v)
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
