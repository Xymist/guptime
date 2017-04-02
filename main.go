package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/boltdb/bolt"
	ping "github.com/sparrc/go-ping"
)

func main() {

	// Database to keep all the status changes in
	db, err := bolt.Open("uptime.db", 0600, nil)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer db.Close()

	// The following use of context is definitely overkill for this purpose...
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	termOrder := make(chan os.Signal, 1)
	terminate := make(chan bool)
	signal.Notify(termOrder, os.Interrupt)

	go func() {
		for _ = range termOrder {
			fmt.Println("Terminating")
			cancel()
			terminate <- true
		}
	}()

	notifyChan := make(chan int64)

	go logUptime(ctx, db, notifyChan)
	go server(db, notifyChan)

	<-terminate
}

func logUptime(ctx context.Context, db *bolt.DB, notifyChan chan int64) {
	var lastStatus bool

	db.Update(func(tx *bolt.Tx) error {
		// Create our bucket to store the changes of state, so we have some
		// persistence between runs and can do more complex things with the data
		// later on.
		_, err := tx.CreateBucketIfNotExists([]byte("stateChanges"))
		if err != nil {
			return fmt.Errorf("Could not find or create bucket: %s", err)
		}
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			// We're done, either because the page was closed or we got a SIGTERM
			return
		case <-time.After(500 * time.Millisecond): // Maybe don't hardcode this?
			status := checkNetPresence("8.8.8.8")
			if status != lastStatus {
				timestamp := time.Now().Unix()

				db.Update(func(tx *bolt.Tx) error {
					b := tx.Bucket([]byte("stateChanges"))
					err := b.Put([]byte(fmt.Sprint(timestamp)), []byte(fmt.Sprint(status)))
					return err
				})
				// Cheaty use of select to make this nonblocking; we don't care if nobody
				// is accessing the web interface
				select {
				case notifyChan <- timestamp:
					lastStatus = status
				default:
					lastStatus = status
				}
			}
		}
	}
}

// We're not really logging uptime, here, just increases in latency to over 1000
// ms. It would probably be a good idea to find a more canonical source for the
// presence or absence of an internet connection than 'how long does it take to
// ping Google?'.
func checkNetPresence(ip string) bool {
	var up bool
	pinger, err := ping.NewPinger(ip)
	if err != nil {
		log.Fatalf("Total failure: %s", err.Error())
	}

	// Only try once before notifying upstream
	pinger.Count = 1
	// A second should do, adjust if necessary
	pinger.Timeout = 1000 * time.Millisecond

	pinger.OnRecv = func(pkt *ping.Packet) {
		// All we need to do here is notify.
		up = true
	}

	// Run once, return whatever we get.
	pinger.Run()
	return up
}
