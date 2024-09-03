package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

var (
	temperatureGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pi_temperature",
			Help: "The current temperature of the Raspberry Pi in degrees Celsius.",
		},
		[]string{"name"},
	)

	tempFile    string
	metricsPath string
	name        string
	interval    time.Duration
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(temperatureGauge)
}

func main() {
	metricsPath = "/metrics"
	val, ok := os.LookupEnv("METRICS_PATH")
	if ok {
		metricsPath = "/" + strings.TrimPrefix(val, "/")
	}

	name = ""
	val, ok = os.LookupEnv("NAME")
	if ok {
		name = val
	}

	tempFile = "sys/class/thermal/thermal_zone0/temp"
	val, ok = os.LookupEnv("TEMP_FILE")
	if ok {
		tempFile = val
	}

	interval = 10 * time.Second
	val, ok = os.LookupEnv("INTERVAL")
	if ok {
		var err error
		interval, err = time.ParseDuration(val)
		if err != nil {
			log.Fatal().Err(err).Msg("error parsing interval")
		}
	}

	debugMode := false
	val, ok = os.LookupEnv("DEBUG")
	if ok && val == "true" {
		debugMode = true
	}
	if debugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	port := ":8080"
	val, ok = os.LookupEnv("PORT")
	if ok {
		port = ":" + strings.TrimPrefix(val, ":")
	}

	router := gin.Default()
	router.GET(metricsPath, gin.WrapH(promhttp.Handler()))

	start(interval)

	log.Debug().Str("port", port).Msg("Starting server")
	err := router.Run(port)
	if err != nil {
		log.Fatal().Err(err).Msg("error starting server")
	}
}

func start(interval time.Duration) {
	log.Info().Str("interval", interval.String()).Msg("Starting temperature monitor")
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			updateTemperature()
		}
	}()
}

func updateTemperature() {
	tempData, err := os.ReadFile(tempFile)
	if err != nil {
		log.Error().Err(err).Msg("error reading temperature file")
		return
	}

	tempString := strings.TrimSuffix(string(tempData), "\n")
	tempInt, err := strconv.Atoi(tempString)
	if err != nil {
		log.Error().Err(err).Msg("error converting temperature to int")
		return
	}

	temp := float64(tempInt) / 1000.0
	log.Debug().Float64("temp", temp).Msg("Updating temperature")
	temperatureGauge.WithLabelValues(name).Set(temp)
}
