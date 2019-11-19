package main

import (
	"strconv"
	"strings"
	"sync"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace string = "ssdb"
)

func init() {
	prometheus.MustRegister(NewSSDBExporter())
}

func parseFloatOrZero(s string) float64 {
	res, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return res
}

type SSDBMetric struct {
	desc    *prometheus.Desc
	valType prometheus.ValueType
}

type SSDBExporter struct {
	mutex   sync.RWMutex
	up      *prometheus.Desc
	metrics map[string]SSDBMetric
}

func NewSSDBExporter() *SSDBExporter {
	return &SSDBExporter{
		up: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "up"), "Exporter successful", []string{"addr"}, nil),
		metrics: map[string]SSDBMetric{
			"db_size": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "db_size"), "The estimated size of the database (possibly different from the hard disk usage) in bytes. If the server has compression enabled, this size is the compressed size.", []string{"addr"}, nil),
				valType: prometheus.GaugeValue,
			},
			"links": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "links"), "The number of connections to the current server.", []string{"addr"}, nil),
				valType: prometheus.CounterValue,
			},
			"command_call_total": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "command_call_total"), "Total command call.", []string{"addr", "command"}, nil),
				valType: prometheus.CounterValue,
			},
			"command_time_wait_total": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "command_time_wait_total"), "The time a command waits for processing", []string{"addr", "command"}, nil),
				valType: prometheus.CounterValue,
			},
			"command_time_proc_total": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "command_time_proc_total"), "The time consumed by the command execution.", []string{"addr", "command"}, nil),
				valType: prometheus.CounterValue,
			},
		},
	}
}

func (e *SSDBExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	for _, m := range e.metrics {
		ch <- m.desc
	}

}

func parserSSDBInfo(data []string) (db_size, links string, call_total, time_wait, time_proc map[string]string, ok bool) {
	ok = false
	if len(data) == 0 {
		return "", "", nil, nil, nil, ok
	}

	if data[0] == "ok" {
		ok = true
	}

	db_size = data[9]
	links = data[5]
	call_total = make(map[string]string)
	time_wait = make(map[string]string)
	time_proc = make(map[string]string)

	for i := 14; i < len(data); i++ {
		command := data[i]
		call := strings.Split(strings.Split(data[i+1], "\t")[0], " ")[1]
		call_time_wait := strings.Split(strings.Split(data[i+1], "\t")[1], " ")[1]
		call_time_proxy := strings.Split(strings.Split(data[i+1], "\t")[2], " ")[1]
		call_total[command] = call
		time_wait[command] = call_time_wait
		time_proc[command] = call_time_proxy
		i++
	}

	return
}

func (e *SSDBExporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	wait := sync.WaitGroup{}
	for _, addr := range strings.Split(*ssdbAddrList, ",") {
		wait.Add(1)
		go func(addr string) {
			defer wait.Done()
			zk_status := 1

			data, ok := sendSSDBCommand(addr)
			if !ok {
				zk_status = 0
			}

			db_size, links, command_call_total, command_time_wait, command_time_proc, ok2 := parserSSDBInfo(data)
			if !ok2 {
				zk_status = 0
			}
			// "db_size": "xx",
			// "links": "xx",
			// "command_call_total": {
			//    "command_x": "xxx",
			// },
			// "command_time_wait": {
			//     "command_x": "xxx",
			// },
			// "command_time_proc": {
			//     "command_x": "xxx",
			// }
			ch <- prometheus.MustNewConstMetric(e.metrics["db_size"].desc, e.metrics["db_size"].valType, parseFloatOrZero(db_size), addr)
			ch <- prometheus.MustNewConstMetric(e.metrics["links"].desc, e.metrics["links"].valType, parseFloatOrZero(links), addr)
			for k, v := range command_call_total {
				ch <- prometheus.MustNewConstMetric(e.metrics["command_call_total"].desc, e.metrics["command_call_total"].valType, parseFloatOrZero(v), addr, k)
			}
			for k, v := range command_time_wait {
				ch <- prometheus.MustNewConstMetric(e.metrics["command_time_wait_total"].desc, e.metrics["command_time_wait_total"].valType, parseFloatOrZero(v), addr, k)
			}
			for k, v := range command_time_proc {
				ch <- prometheus.MustNewConstMetric(e.metrics["command_time_proc_total"].desc, e.metrics["command_time_proc_total"].valType, parseFloatOrZero(v), addr, k)
			}

			ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, float64(zk_status), addr)
		}(addr)
	}
	wait.Wait()
	e.mutex.Unlock()
}
