# fishy

* This project is v2 of [Tatsumaki](https://tatsumaki.xyz)'s fishy suite of commands  
* As the project progresses more documentation will become available for people looking to run this on their own.  
* note: you will need to create your own fish.json and items.json files, as those are not being made public if you wish to run this on your own. I will not provide any support for people wishing to run this on their own, and is being released as-is. Meaning, this *may* or *may not* work for your specific use case

# info

* This is a separate API server programmed entirely in Go  
* A combination of REST routes and a websocket are used to talk with the main bot  
* The REST routes are used for interfacing with the fishy database and the websocket is used to modify credits on Tatsumaki's database when needed

# requirements
* Go, preferrably 1.8 or above
* `github.com/go-redis/redis`, `github.com/iopred/discordgo`, `github.com/gorilla/mux`, `github.com/gorilla/websocket`, `github.com/mitchellh/mapstructure`
* A Redis database

# contributors
* [thy](https://github.com/ThyLeader)
* [ode](https://github.com/odevine)
* maybe you ;)

# want to contribute?
* Fork this repository, commit, and then send a pull request to the Dev branch. All PRs pointing to the master branch will be closed.
* Run your code through `go fmt` and `go vet` before issuing a PR
