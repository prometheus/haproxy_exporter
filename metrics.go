package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type MetricFamily int

const (
	Counter MetricFamily = iota
	Gauge
)

// HAProxy 1.4
// pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,
// HAProxy 1.5
// pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,comp_in,comp_out,comp_byp,comp_rsp,lastsess,
// HAProxy 1.6
// pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,comp_in,comp_out,comp_byp,comp_rsp,lastsess,last_chk,last_agt,qtime,ctime,rtime,ttime,

const (
	namespace             = "haproxy" // For Prometheus metrics.
	expectedCsvFieldCount = 52
	statusField           = 17
)

type metricSpec struct {
	metricName   string
	docString    string
	metricFamily MetricFamily
	metricLabels prometheus.Labels
}

var (
	metricSpecs = []metricSpec{
		// 0
		{},
		{},
		{
			// qcur
			"current_queue",
			"Current number of queued requests assigned to this server",
			Gauge,
			nil,
		},
		{
			// qmax
			"max_queue",
			"Maximum observed number of queued requests assigned to this server.",
			Gauge,
			nil,
		},
		{
			// scur
			"current_sessions",
			"Current number of active sessions.",
			Gauge,
			nil,
		},

		// 5
		{
			// smax
			"max_sessions",
			"Maximum observed number of active sessions.",
			Gauge,
			nil,
		},
		{
			// slim
			"limit_sessions",
			"Configured session limit.",
			Gauge,
			nil,
		},
		{
			// stot
			"connections_total",
			"Total number of connections.",
			Counter,
			nil,
		},
		{
			// bin
			"bytes_in_total",
			"Current total of incoming bytes.",
			Counter,
			nil,
		},
		{
			// bout
			"bytes_out_total",
			"Current total of outgoing bytes.",
			Counter,
			nil,
		},

		// 10
		{
			// dreq
			"requests_denied_total",
			"Total of requests denied for security.",
			Counter,
			nil,
		},
		{
		// dresp
		},
		{
			// ereq
			"request_errors_total",
			"Total of request errors.",
			Counter,
			nil,
		},
		{
			// econ
			"connection_errors_total",
			"Total of connection errors.",
			Counter,
			nil,
		},
		{
			// eresp
			"response_errors_total",
			"Total of response errors.",
			Counter,
			nil,
		},

		// 15
		{
			// wretr
			"retry_warnings_total",
			"Total of retry warnings.",
			Counter,
			nil,
		},
		{
			// wredis
			"redispatch_warnings_total",
			"Total of redispatch warnings.",
			Counter,
			nil,
		},
		{
			// status
			"up",
			"Current health status of the server (1 = UP, 0 = DOWN }.",
			Gauge,
			nil,
		},
		{
			// weight
			"weight",
			"Current weight of the server.",
			Gauge,
			nil,
		},
		{
		// act
		},

		// 20
		{
		// bck
		},
		{
			// chkfail
			"check_failures_total",
			"Total number of failed health checks.",
			Counter,
			nil,
		},
		{
		// chkdown
		},
		{
		// lastchg
		},
		{
			// downtime
			"downtime_seconds_total",
			"Total downtime in seconds.",
			Counter,
			nil,
		},

		// 25
		{
		// qlimit
		},
		{
		// pid
		},
		{
		// iid
		},
		{
		// sid
		},
		{
		// throttle
		},

		// 30
		{
		// lbtot
		},
		{
		// tracked
		},
		{
		// type
		},
		{
			// rate
			"current_session_rate",
			"Current number of sessions per second over last elapsed second.",
			Gauge,
			nil,
		},
		{
			// rate_lim
			"limit_session_rate",
			"Configured limit on new sessions per second.",
			Gauge,
			nil,
		},

		// 35
		{
			// rate_max
			"max_session_rate",
			"Maximum observed number of sessions per second.",
			Gauge,
			nil,
		},
		{
		// check_status
		},
		{
		// check_code
		},
		{
			// check_duration
			"check_duration_milliseconds",
			"Previously run health check duration, in milliseconds",
			Gauge,
			nil,
		},
		{
			// hrsp_1xx
			"http_responses_total",
			"Total of HTTP responses.",
			Counter,
			prometheus.Labels{"code": "1xx"},
		},

		// 40
		{
			// hrsp_2xx
			"http_responses_total",
			"Total of HTTP responses.",
			Counter,
			prometheus.Labels{"code": "2xx"},
		},
		{
			// hrsp_3xx
			"http_responses_total",
			"Total of HTTP responses.",
			Counter,
			prometheus.Labels{"code": "3xx"},
		},
		{
			// hrsp_4xx
			"http_responses_total",
			"Total of HTTP responses.",
			Counter,
			prometheus.Labels{"code": "4xx"},
		},
		{
			// hrsp_5xx
			"http_responses_total",
			"Total of HTTP responses.",
			Counter,
			prometheus.Labels{"code": "5xx"},
		},
		{
			// hrsp_other
			"http_responses_total",
			"Total of HTTP responses.",
			Counter,
			prometheus.Labels{"code": "other"},
		},

		// 45
		{
		// hanafail
		},
		{
		// req_rate
		},
		{
		// req_rate_max
		},
		{
			// req_tot
			"http_requests_total",
			"Total HTTP requests.",
			Counter,
			nil,
		},
		{
		// cli_abrt
		},

		// 50
		{
		// srv_abrt
		},
		{
		// comp_in
		},
		{
		// comp_out
		},
		{
		// comp_byp
		},
		{
		// comp_rsp
		},

		// 55
		{
		// lastsess
		},
		{
		// last_chk
		},
		{
		// last_agt
		},
		{
		// qtime
		},
		{
		// ctime
		},

		// 60
		{
		// rtime
		},
		{
		// ttime
		},
	}
)
