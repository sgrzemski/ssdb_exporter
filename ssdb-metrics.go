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
	desc          *prometheus.Desc
	extract       func(interface{}) float64
	extractLabels func(s interface{}) []string
	valType       prometheus.ValueType
}

func NewSSDBMetric(metricName, subSystem, docString string, valType prometheus.ValueType) *SSDBMetric {
	return &SSDBMetric{}
}

type SSDBExporter struct {
	mutex   sync.RWMutex
	up      *prometheus.Desc
	metrics map[string]SSDBMetric
}

func NewSSDBExporter() *SSDBExporter {
	return &SSDBExporter{
		up: prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "up"), "Exporter successful", nil, nil),
		metrics: map[string]SSDBMetric{
			"db_size": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "db_size"), "数据库估计的大小(可能和硬盘占用差异非常大, 以 du 命令显示的为准), 字节数. 如果服务器开启了压缩, 这个大小是压缩后的大小.", nil, nil),
				extract: func(s interface{}) float64 { return parseFloatOrZero(s.(string)) },
				valType: prometheus.GaugeValue,
			},
			"links": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "links"), "当前服务器的连接数.", nil, nil),
				extract: func(s interface{}) float64 { return parseFloatOrZero(s.(string)) },
				valType: prometheus.CounterValue,
			},
			"command_call_total": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "command_call_total"), "命令执行次数.", []string{"command"}, nil),
				extract: func(s interface{}) float64 { return parseFloatOrZero(s.(string)) },
				extractLabels: func(s interface{}) []string {
					return []string{s.(string)}
				},
				valType: prometheus.CounterValue,
			},
			"command_time_wait_total": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "command_time_wait_total"), "命令等待处理的时间.", []string{"command"}, nil),
				extract: func(s interface{}) float64 { return parseFloatOrZero(s.(string)) },
				extractLabels: func(s interface{}) []string {
					return []string{s.(string)}
				},
				valType: prometheus.CounterValue,
			},
			"command_time_proc_total": SSDBMetric{
				desc:    prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "command_time_proc_total"), "命令执行消耗的时间.", []string{"command"}, nil),
				extract: func(s interface{}) float64 { return parseFloatOrZero(s.(string)) },
				extractLabels: func(s interface{}) []string {
					return []string{s.(string)}
				},
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
	zk_status := 1

	data, ok := sendSSDBCommand()
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
	ch <- prometheus.MustNewConstMetric(e.metrics["db_size"].desc, e.metrics["db_size"].valType, e.metrics["db_size"].extract(db_size))
	ch <- prometheus.MustNewConstMetric(e.metrics["links"].desc, e.metrics["links"].valType, e.metrics["links"].extract(links))
	for k, v := range command_call_total {
		ch <- prometheus.MustNewConstMetric(e.metrics["command_call_total"].desc, e.metrics["command_call_total"].valType, e.metrics["command_call_total"].extract(v), e.metrics["command_call_total"].extractLabels(k)...)
	}
	for k, v := range command_time_wait {
		ch <- prometheus.MustNewConstMetric(e.metrics["command_time_wait_total"].desc, e.metrics["command_time_wait_total"].valType, e.metrics["command_time_wait_total"].extract(v), e.metrics["command_time_wait_total"].extractLabels(k)...)
	}
	for k, v := range command_time_proc {
		ch <- prometheus.MustNewConstMetric(e.metrics["command_time_proc_total"].desc, e.metrics["command_time_proc_total"].valType, e.metrics["command_time_proc_total"].extract(v), e.metrics["command_time_proc_total"].extractLabels(k)...)
	}

	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, float64(zk_status))
}
