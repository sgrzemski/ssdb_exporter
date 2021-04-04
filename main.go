package main

import (
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
)

func init() {
	flag.Parse()
	getEnvVars()

	parsedLevel, err := log.ParseLevel(*rawLevel)
	if err != nil {
		log.Fatal(err)
	}
	logLevel = parsedLevel

	prometheus.MustRegister(version.NewCollector("ssdb_exporter"))
}

var (
	logLevel     log.Level = log.InfoLevel
	bindAddr               = flag.String("bind-addr", ":9142", "bind address for the metrics server")
	metricsPath            = flag.String("metrics-path", "/metrics", "path to metrics endpoint")
	ssdbAddrList           = flag.String("ssdb-list", "localhost:8888", "host1:port1,host2:port2 for ssdb socket")
	rawLevel               = flag.String("log-level", "info", "log level")
)

func main() {
	log.Info(version.Print("ssdb_exporter"))
	log.SetLevel(logLevel)
	log.Info("Starting ssdb_exporter")

	go serveMetrics()

	exitChannel := make(chan os.Signal)
	signal.Notify(exitChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	exitSignal := <-exitChannel
	log.WithFields(log.Fields{"signal": exitSignal}).Infof("Caught %s signal, exiting", exitSignal)
}

func serveMetrics() {
	log.Infof("Starting metric http endpoint on %s", *bindAddr)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", rootHandler)
	log.Fatal(http.ListenAndServe(*bindAddr, nil))
}
func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<html>
        <head><title>ssdb Exporter</title></head>
        <body>
        <h1>ssdb Exporter</h1>
        <p><a href="` + *metricsPath + `">Metrics</a></p>
        </body>
        </html>`))
}

func getEnvVars() {
	log.Debug("Loading environment variables")
	var envSsdbAddrList = os.Getenv("SSDB_LIST")
	if len(envSsdbAddrList) > 0 {
		*ssdbAddrList = envSsdbAddrList
		log.Debug("Loaded SSDB_LIST variable")
	}
	var envBindAddr = os.Getenv("BIND_ADDR")
	if len(envBindAddr) > 0 {
		*bindAddr = envBindAddr
		log.Debug("Loaded BIND_ADDR variable")
	}
	var envMetricsPath = os.Getenv("METRICS_PATH")
	if len(envMetricsPath) > 0 {
		*metricsPath = envMetricsPath
		log.Debug("Loaded METRICS_PATH variable")
	}
	var envLogLevel = os.Getenv("LOG_LEVEL")
	if len(envLogLevel) > 0 {
		*rawLevel = envLogLevel
		log.Debug("Loaded LOG_LEVEL variable")
	}
}
