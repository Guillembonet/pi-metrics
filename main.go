package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	name := ""
	val := os.Getenv("NAME")
	if val != "" {
		name = val
	}

	tempFile := "sys/class/thermal/thermal_zone0/temp"
	val = os.Getenv("TEMP_FILE")
	if val != "" {
		tempFile = val
	}

	address := ":8080"
	val = os.Getenv("ADDRESS")
	if val != "" {
		address = val
	}

	registry := prometheus.NewRegistry()
	registry.Register(NewTempCollector(name, tempFile))

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.Handle("GET /health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	server := &http.Server{
		Addr:    address,
		Handler: mux,
	}
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to serve", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shutdown server", slog.String("error", err.Error()))
	}

	slog.Info("stopped gracefully")
}

type TempCollector struct {
	name, tempFile string
	desc           *prometheus.Desc
}

func NewTempCollector(name, tempFile string) *TempCollector {
	return &TempCollector{
		name:     name,
		tempFile: tempFile,
		desc: prometheus.NewDesc("pi_temperature",
			"The current temperature of the Raspberry Pi in degrees Celsius.",
			nil, prometheus.Labels{"name": name},
		),
	}
}

func (t *TempCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- t.desc
}

func (t *TempCollector) Collect(ch chan<- prometheus.Metric) {
	temp, err := getTemperature(t.tempFile)
	if err != nil {
		slog.Error("failed to get temperature", slog.String("error", err.Error()))
		return
	}

	metric, err := prometheus.NewConstMetric(t.desc, prometheus.GaugeValue, temp)
	if err != nil {
		slog.Error("failed to create metric", slog.String("error", err.Error()))
		return
	}

	ch <- metric
}

func getTemperature(tempFile string) (float64, error) {
	tempData, err := os.ReadFile(tempFile)
	if err != nil {
		return 0, fmt.Errorf("read temperature file (%s): %w", tempFile, err)
	}

	tempInt, err := strconv.Atoi(strings.TrimSpace(string(tempData)))
	if err != nil {
		return 0, fmt.Errorf("convert temperature to int: %w", err)
	}

	return float64(tempInt) / 1000.0, nil
}
