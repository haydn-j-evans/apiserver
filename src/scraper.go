package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type taxi struct {
	ID        string
	PosX      int
	PosY      int
	Available bool
}

// Function to initilise the redis pool of connections
func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     80,
		MaxActive:   12000, // max number of connections
		IdleTimeout: 120 * time.Second,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", ":6379")
			if err != nil {
				panic(err.Error())
			}
			return conn, err
		},
	}

}

// Scrape API and store the JSON object returned in the struct "taxi"

func getPositions() {

	contextLogger := log.WithFields(log.Fields{
		"Application_Name": "API_Server",
		"Application_ID":   "A0002",
		"Environment":      environment,
	})

	// Initialise the connection pool to redis

	flag.Parse()
	pool = newPool()
	connections := pool.Get()
	defer connections.Close()

	for {

		url := "http://localhost:8080/positions"
		contextLogger.Info("Querying the API @ ", url)

		// Create HTTP client with timeout

		scrapeClient := &http.Client{
			Timeout: 5 * time.Second,
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			contextLogger.Warn(err)
		}

		// Set Agent Headers
		req.Header.Set("User-Agent", "Position Scraper")

		// Log if error in the reponse
		start := time.Now()
		res, err := scrapeClient.Do(req)
		if err != nil {
			contextLogger.Warn(err)
			time.Sleep(15 * time.Second)
		} else {
			// Log response status and response time

			if res.StatusCode == 200 {
				contextLogger.Info("API Returned Status OK!")
			} else {
				contextLogger.Warn(res.StatusCode)
			}

			elapsed := time.Since(start).Seconds()
			contextLogger.Info("API responded in (s) - ", elapsed)

			// Log if error in reading body
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				contextLogger.Warn(err)
			}

			// create variable "taxis" to store the JSON array unmarshalled by the JSON module as a struct
			var taxis []taxi
			json.Unmarshal(body, &taxis)

			start2 := time.Now()
			contextLogger.Info("Writing Keys to Redis!")

			for p := range taxis {

				if _, err := connections.Do("HSET", redis.Args{}.Add(taxis[p].ID).AddFlat(taxis[p])...); err != nil {
					contextLogger.Warn(err)
					contextLogger.Warn("Could not HSET hash value!")
					break

				}
				if _, err := connections.Do("EXPIRE", taxis[p].ID, 180); err != nil {
					contextLogger.Warn(err)
					contextLogger.Warn("Could not EXPIRE hash value!")
					break
				}

				if _, err := connections.Do("ZADD", "avaliable", taxis[p].Available, taxis[p].ID); err != nil {
					contextLogger.Warn(err)
					contextLogger.Warn("Could not ZADD key value!")
					break
				}

				if _, err := connections.Do("EXPIRE", "avaliable", 180); err != nil {
					contextLogger.Warn(err)
					contextLogger.Warn("Could not EXPIRE ZADD value!")
					break
				}

			}

			elapsed2 := time.Since(start2).Seconds()
			contextLogger.Info("Keys written to Redis in (s) - ", elapsed2)

			contextLogger.Info("Sleeping for 15!")
			time.Sleep(15 * time.Second)

		}

	}

}

func Get() redis.Conn {
	return pool.Get()
}

var pool *redis.Pool
var contextLogger *log.Entry
var environment string

func init() {

	environment = os.Getenv("ENVIRONMENT")

	// Log as JSON instead of the default ASCII formatter.

	elk_logger := os.Getenv("ELK_LOGGER")
	b1, _ := strconv.ParseBool(elk_logger)
	if b1 == true {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{})
	}

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.InfoLevel)

}

func main() {

	//Init Logger in the main function
	contextLogger := log.WithFields(log.Fields{
		"Application_Name": "API_Server",
		"Application_ID":   "A0002",
		"Environment":      environment,
	})

	//Sleep 5 to allow redis to start first

	contextLogger.Info("Starting Application - Waiting for Redis!")
	time.Sleep(5 * time.Second)

	//recordMetrics()

	go getPositions()

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
