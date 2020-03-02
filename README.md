# Random Password Please

*Random Password Please* is a simple Go demo app that generates random passwords.
Using only standard library packages it demonstrates:

* how to write a simple web server
* template parsing
* goroutines and channels
* generating random tokens
* signal handling for graceful shutdowns

The live version is online at https://random-password-please.herokuapp.com/.
(Note that it might take a few seconds to load if the app is asleep.)

## Running Locally

```sh
$ git clone https://github.com/jbarham/random-password-please.git
$ cd random-password-please
$ go run main.go
```

The very basic default page can be replaced by adding a
[Go template file](http://golang.org/pkg/text/template/)
named `index.html` in the same directory as the executable.

## Deploying to Heroku

```sh
$ heroku create
$ git push heroku master
$ heroku open
```

### Author

John Barham, [Wombat Software](https://www.wombatsoftware.com/)
