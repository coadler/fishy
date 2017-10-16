package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ThyLeader/fishy/database"
	"github.com/ThyLeader/fishy/routes"

	"github.com/ThyLeader/discordrus"
	"github.com/buaazp/fasthttprouter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"
	"gopkg.in/olivere/elastic.v5"
	"gopkg.in/sohlich/elogrus.v2"
)

// Version of application, displayed in `Server` header in all responses.
const Version = "0.1.0"

// Configuration files map to paths.
var configFiles = map[string]string{
	"config":        ".",
	"fish":          "./json",
	"items":         "./json",
	"levels":        "./json",
	"secretstrings": "./json",
	"trash":         "./json",
}

func init() {
	// Load individual files into configuration
	for k, f := range configFiles {
		v := viper.New()
		v.SetConfigType("json")
		v.SetConfigName(k)
		v.AddConfigPath(f)
		if err := v.ReadInConfig(); err != nil {
			log.WithFields(log.Fields{
				"err":  err,
				"key":  k,
				"file": fmt.Sprintf("%s/%s.json", f, k),
			}).Fatal("failed to read in a configuration file")
			return
		}
		viper.Set(k, v.AllSettings())
	}

	// Configure logrus hooks
	if viper.GetBool("config.logging.elastic.enabled") {
		url := viper.GetString("config.logging.elastic.url")
		host := viper.GetString("config.logging.elastic.host")
		index := viper.GetString("config.logging.elastic.index")

		client, err := elastic.NewClient(elastic.SetURL(url))
		if err != nil {
			log.WithField("err", err).Fatal("failed to create elastic client for elogrus")
			return
		}
		hook, err := elogrus.NewElasticHook(client, host, log.DebugLevel, index)
		if err != nil {
			log.WithField("err", err).Fatal("failed to create elogrus hook")
			return
		}
		log.AddHook(hook)
	}
	if viper.GetBool("config.logging.discord.enabled") {
		log.AddHook(discordrus.NewHook(
			viper.GetString("config.logging.discord.webhook"),
			log.ErrorLevel,
			&discordrus.Opts{
				Username:           viper.GetString("config.logging.discord.username"),
				Author:             "",
				DisableTimestamp:   false,
				TimestampFormat:    "Jan 2 15:04:05.00000",
				EnableCustomColors: true,
				CustomLevelColors: &discordrus.LevelColors{
					Debug: 10170623,
					Info:  3581519,
					Warn:  14327864,
					Error: 13631488,
					Panic: 13631488,
					Fatal: 13631488,
				},
				DisableInlineFields: true,
			},
		))
	}

	// Enable debug logging mode if DEBUG environment variable is truthy
	if strconv.ParseBool(os.Getenv("DEBUG")) {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	log.Info("dean") // never remove this line

	// Connect to redis
	err := database.Init(
		viper.GetString("config.redis.url"),
		viper.GetString("config.redis.password"),
		viper.GetInt("config.redis.db"),
	)
	if err != nil {
		log.WithField("err", err).Fatal("failed to ping redis database")
		return
	}

	// Initialize fasthttprouter
	router := fasthttprouter.Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          false,
		NotFound:               routes.NotFound,
		MethodNotAllowed:       routes.MethodNotAllowed,
		PanicHandler:           routes.PanicHandler,
	}

	// Add routes to router
	router.GET("/v1", logWrap("Index", routes.Index))
	router.POST("/v1/fish/:guildID", logWrap("Fish", routes.Fish))

	// Create fasthttp server
	server := &fasthttp.Server{
		Handler:      router.Handler,
		Name:         fmt.Sprintf("fishy/v%s", Version),
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 30 * time.Minute,
		LogAllErrors: log.Level == log.DebugLevel,
		Logger:       log,
	}

	// Listen for requests
	log.WithField("addr", viper.GetString("config.listenAddr")).Info("attempting to listen for requests")
	if err = server.ListenAndServe(viper.GetString("config.listenAddr")); err != nil {
		log.WithFields(log.Fields{
			"addr": viper.GetString("config.listenAddr"),
			"err":  err,
		}).Error("error in &(fasthttp.Server).ListenAndServe")
	}
	log.Info("server shut down")
}

func logWrap(name string, handler func(*fasthttp.RequestCtx)) func(*fasthttp.RequestCtx) {
	return func(ctx *fastthtp.RequestCtx) {
		// Time request
		start := time.Now()
		handler(ctx)
		elapsed := time.Since(start)

		// Log request
		log.WithFields(log.Fields{
			"method":      string(r.Method),
			"request_uri": string(r.RequestURI),
			"name":        name,
			"elapsed":     elapsed,
		}).Info("request")
	}
}
