package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/influxdata/influxdb-client-go"
	"io"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"
)

type RequestInfo struct {
	Method string
	Path string
	Schema string
}

type AccessInfo struct {
	AccessIp string
	AccessUser string
	TimeLocal time.Time
	RequestInfo RequestInfo
	Status int
	BodyBytesSend int
	HttpReferrer string
	HttpUserAgent string
}

type LogProcess struct {
	rc chan []byte
	wc chan *AccessInfo
	read Reader
	write Write
}

type Reader interface {
	Read(rc chan []byte)
}

type Write interface {
	Write(wc chan *AccessInfo)
}

type ReadFromFile struct {
	path string
}

type InfluxDBConf struct {
	addr, username, password, token string
}

type Write2DB struct {
	dbConf InfluxDBConf
}

func (r *ReadFromFile) Read(rc chan []byte)  {
	file, err := os.Open(r.path)
	if err != nil {
		panic(fmt.Sprintf("open file error: %s", err.Error()))
	}
	defer file.Close()
	file.Seek(0, 2)

	rd := bufio.NewReader(file)

	for {
		line, err := rd.ReadBytes('\n')

		if err == io.EOF {
			time.Sleep(500 * time.Millisecond)
			continue
		} else if err != nil {
			panic(fmt.Sprintf("read error: %s", err.Error()))
		}

		rc <- line[:len(line) - 1]
	}
}

func (w *Write2DB) Write(wc chan *AccessInfo)  {
	for data := range wc {
		influx, err := influxdb.New(w.dbConf.addr, w.dbConf.token)
		if err != nil {
			panic(err) // error handling here; normally we wouldn't use fmt but it works for the example
		}

		// we use client.NewRowMetric for the example because it's easy, but if you need extra performance
		// it is fine to manually build the []client.Metric{}.
		myMetrics := []influxdb.Metric{
			influxdb.NewRowMetric(
				map[string]interface{}{
					"access_path": data.RequestInfo.Path,
					"body_bytes_send": data.BodyBytesSend,
					"http_user_agent": data.HttpUserAgent,
					"http_referrer": data.HttpReferrer,
					"access_user": data.AccessUser,
					"access_status": string(data.Status),
				},
				"nginx_log",
				map[string]string{
					"access_ip": data.AccessIp,
					"access_method": data.RequestInfo.Method,
					"access_schema": data.RequestInfo.Schema,
				},
				data.TimeLocal),
		}

		fmt.Println(data.TimeLocal.Format("2006-01-02 15:04:05"))
		// The actual write..., this method can be called concurrently.
		if err := influx.Write(context.Background(), "nginx", "admin", myMetrics...); err != nil {
			log.Fatal(err) // as above use your own error handling here.
		}
		influx.Close() // closes the client.  After this the client is useless.
	}
}

func (lp *LogProcess) Process()  {
	var matches []string

	/*
	100.116.222.152 - - [19/Sep/2018:15:28:14 +0800] "GET /api/child_star/query?classify=2&page=1&page_size=18 HTTP/1.1" 301 178 "-" "okhttp/3.10.0"
	*/
	regStr := `([\d\.]+)\s-\s(.*?)\s\[(.*?)\]\s"(.*?)\s(.*?)\s(.*?)"\s(\d+)\s(\d+)\s"(.*?)"\s"(.*?)"`
	reg := regexp.MustCompile(regStr)

	timeLocation, _ := time.LoadLocation("Asia/Shanghai")
	for data := range lp.rc {
		line := string(data)
		matches = reg.FindStringSubmatch(line)

		if len(matches) < 11 {
			log.Println("match fail: ", len(matches), matches, line, regStr)
			continue
		}

		timeLocal, err := time.ParseInLocation("02/Jan/2006:15:04:05 +0800", matches[3], timeLocation)
		if err != nil {
			log.Println("parse timeLocal fail: ", matches[3], err.Error())
			continue
		}

		status, err := strconv.Atoi(matches[7])
		if err != nil {
			log.Println("status convert to int fail: ", status, err.Error())
			continue
		}

		bodyBytesSend, err := strconv.Atoi(matches[8])
		if err != nil {
			log.Println("bodyBytesSend convert to int fail: ", bodyBytesSend, err.Error())
			continue
		}

		pathStr := matches[5]

		path, err := url.Parse(pathStr)
		if err != nil {
			log.Println("parse path fail: ", pathStr, err.Error())
			continue
		}

		accessInfo := &AccessInfo{
			AccessIp:   matches[1],
			AccessUser: matches[2],
			TimeLocal:  timeLocal,
			RequestInfo: RequestInfo{
				Method: matches[4],
				Path:   path.Path,
				Schema: matches[6],
			},
			Status:        status,
			BodyBytesSend: bodyBytesSend,
			HttpReferrer:  matches[9],
			HttpUserAgent: matches[10],
		}

		lp.wc <- accessInfo
	}
}

func main() {
	defer func() {
		if data := recover(); data != nil {
			log.Println("panic error on main", data)
		}
	}()

	r := &ReadFromFile{
		path: "./access.log",
	}

	w := &Write2DB{
		dbConf: InfluxDBConf{
			addr:     "http://192.168.99.100:9999",
			token:    "pF12bN1tmVY_eCJW5TZLGa2ap9TRmEK-F4Df5LyRImhn8xa6qR82x3TyLLPvEQAsQWLsh00EanM1fx6Ugyh8yg==",
		},
	}
	lp := &LogProcess{
		rc:    make(chan []byte),
		wc:    make(chan *AccessInfo),
		read:  r,
		write: w,
	}

	go lp.read.Read(lp.rc)
	go lp.Process()
	go lp.write.Write(lp.wc)

	for {
		select {
		case <- time.After(1 * time.Second):
		}
	}
}
