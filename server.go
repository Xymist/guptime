package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
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
	http.HandleFunc("/", mainPage)
	log.Fatal(http.ListenAndServe("127.0.0.1:9000", nil))
}

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

			status := string(getStatus(db, timestamp))
			if status == "true" {
				status = "came up."
			}
			if status == "false" {
				status = "went down."
			}
			time := stampToTime(timestamp)
			writer.Write([]byte(fmt.Sprintf("At %v the connection %s", time, status)))
			if err := writer.Close(); err != nil {
				return
			}
		}
	}
}

func stampToTime(timestamp int64) time.Time {
	i, err := strconv.ParseInt(strconv.Itoa(int(timestamp)), 10, 64)
	if err != nil {
		panic(err)
	}
	return time.Unix(i, 0)
}

func getStatus(db *bolt.DB, timestamp int64) (value []byte) {
	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("stateChanges"))
		value = bkt.Get([]byte(fmt.Sprint(timestamp)))
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
	http.ServeFile(w, r, "home.html")
}
