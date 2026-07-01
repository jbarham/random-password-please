package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 30
)

var (
	httpAddr = flag.String("http", defaultAddr(), "http listen address")

	// Counts number of passwords generated.
	counter     uint64
	counterLock sync.Mutex // Overkill?

	// Optional file to load/save counter value.
	counterFilePath = flag.String("counter", "", "password counter file")
	counterFile     *os.File
	counterFileLock sync.Mutex

	//go:embed index.html
	indexHTML string
	index     *template.Template

	passwords chan string
)

type indexParams struct {
	Password, Counter, Host string
}

func main() {
	flag.Parse()

	index = template.Must(template.New("index").Parse(indexHTML))

	if *counterFilePath != "" {
		var err error

		counterFile, err = os.OpenFile(*counterFilePath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalf("Failed to open counter file: %s", err)
		}
		counterBytes, err := io.ReadAll(counterFile)
		if err != nil {
			log.Fatalf("Failed to read counter file: %s", err)
		}
		if len(counterBytes) > 0 {
			counter, err = strconv.ParseUint(string(bytes.TrimSpace(counterBytes)), 10, 64)
			if err != nil {
				log.Fatal("Failed to read counter value")
			}
		}
	}

	http.HandleFunc("GET /{$}", indexHandler)

	http.HandleFunc("GET /password.txt", apiHandler)

	http.HandleFunc("GET /counter", counterHandler)

	// Ensure counter is saved on exit.
	go handleSignals()

	go generatePasswords()

	log.Print("Running at address ", *httpAddr)
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		log.Fatal(err)
	}
}

func indexHandler(w http.ResponseWriter, req *http.Request) {
	params := indexParams{
		Password: getPassword()[:minPasswordLength],
		Counter:  fmt.Sprint(counter),
		Host:     req.Host,
	}
	w.Header().Set("Cache-Control", "no-cache")
	index.Execute(w, params)
}

func apiHandler(w http.ResponseWriter, req *http.Request) {
	n := minPasswordLength
	n, err := strconv.Atoi(req.FormValue("len"))
	if err != nil {
		n = minPasswordLength
	} else {
		n = min(max(n, minPasswordLength), maxPasswordLength)
	}
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, getPassword()[:n])
}

func counterHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	s := strconv.FormatUint(counter, 10)
	fmt.Fprint(w, s)
}

func generatePasswords() {
	// Create a buffer of passwords so requests don't have to wait for a password to be generated.
	passwords = make(chan string, 10)

	// Derived from https://docs.djangoproject.com/en/dev/topics/auth/#django.contrib.auth.models.UserManager.make_random_password
	alphabet := "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	password := make([]byte, maxPasswordLength)
	for {
		for i := 0; i < len(password); i++ {
			password[i] = alphabet[rand.Int()%len(alphabet)]
		}
		passwords <- string(password)
	}
}

func getPassword() string {
	counterLock.Lock()
	defer counterLock.Unlock()
	counter++
	if counterFile != nil && counter%100 == 0 {
		go saveCounter()
	}
	return <-passwords
}

func saveCounter() {
	if counterFile == nil {
		return
	}

	counterFileLock.Lock()
	defer counterFileLock.Unlock()

	var err error

	if _, err = counterFile.Seek(0, 0); err == nil {
		if _, err = fmt.Fprint(counterFile, counter); err == nil {
			err = counterFile.Sync()
		}
	}
	if err != nil {
		// Complain, but doesn't seem worth bailing at this point.
		log.Print("Failed to write counter:", err)
	}
}

func handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	log.Printf("Got signal %s, shutting down...", <-sigChan)
	saveCounter()
	os.Exit(0)
}

func defaultAddr() string {
	port := os.Getenv("PORT")
	if port != "" {
		return ":" + port
	}

	return ":8080"
}
