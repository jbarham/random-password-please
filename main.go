package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"text/template"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 30
)

var (
	httpAddr = flag.String("http", ":8080", "http listen address")

	// Counts number of passwords generated.
	counter     uint64
	counterLock sync.Mutex // Overkill?

	// Optional file to load/save counter value.
	counterFilePath = flag.String("counter", "", "password counter file")
	counterFile     *os.File
	counterFileLock sync.Mutex

	index *template.Template

	passwords chan (string)
)

type indexParams struct {
	Password, Counter, Host string
}

func main() {
	flag.Parse()

	if *counterFilePath != "" {
		var err error

		counterFile, err = os.OpenFile(*counterFilePath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalf("Failed to open counter file: %s", err)
		}
		counterBytes, err := ioutil.ReadAll(counterFile)
		if err != nil {
			log.Fatalf("Failed to read counter file: %s", err)
		}
		if len(counterBytes) > 0 {
			counter, err = strconv.ParseUint(string(counterBytes), 10, 64)
			if err != nil {
				log.Fatal("Failed to read counter value")
			}
		}
	}

	http.HandleFunc("/", indexHandler)

	http.HandleFunc("/password.txt", apiHandler)

	http.HandleFunc("/counter", counterHandler)

	// Ensure counter is saved on exit.
	go handleSignals()

	go generatePasswords()

	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}

func indexHandler(w http.ResponseWriter, req *http.Request) {
	params := indexParams{
		Password: getPassword()[:minPasswordLength],
		Counter:  fmt.Sprint(counter),
		Host:     req.Host,
	}
	index.Execute(w, params)
}

func apiHandler(w http.ResponseWriter, req *http.Request) {
	n := minPasswordLength
	n, err := strconv.Atoi(req.FormValue("len"))
	if err != nil {
		n = minPasswordLength
	} else if n < minPasswordLength {
		n = minPasswordLength
	} else if n > maxPasswordLength {
		n = maxPasswordLength
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, getPassword()[:n])
}

func counterHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, counter)
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
		log.Print("Failed to write counter: %s", err)
	}
}

func handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Kill, os.Interrupt)
	<-sigChan
	saveCounter()
	os.Exit(0)
}

func init() {
	var err error

	// Parse optional on-disk index file.
	if index, err = template.ParseFiles("./index.html"); err != nil {
		log.Println(err)
		log.Println("Using default template")
		index = template.Must(template.New("index").Parse(indexHtml))
	}
}

var indexHtml = `
<!doctype html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Random Password Please</title>
</head>
<body>
	<div style="text-align: center">
		<p>Your random password is:</p>
		<h1 id="password">{{.Password}}</h1>
		<form id="form" method="post">
			<button type="submit">Another Password Please</button>
		</form>
		<p>
			<small>
				<span id="counter">{{.Counter}}</span> passwords generated
				<br><attr title="{{.Host}}/password.txt[?len=8-30]">API</attr>
				<br><a href="https://github.com/jbarham/random-password-please">Source</a>
			</small>
		</p>
	</div>
	<script src="http://ajax.googleapis.com/ajax/libs/jquery/1.7.2/jquery.min.js"></script>
	<script type="text/javascript">
		$(document).ready(function(){
			$('#form').submit(function(event) {
				event.preventDefault();
				/* Load new password via API. */
				$('#password').load('/password.txt');
				$('#counter').load('/counter');
			})
		});
	</script>
</body>
</html>
`
