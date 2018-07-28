package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"log"
	"time"
	"github.com/wheresalice/rtt_exporter/rtt"
	"io/ioutil"
	"strconv"
	"os"
	"fmt"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

var (
	stationDepartureDelay = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "wheresalice",
			Subsystem: "rtt",
			Name: "average_train_departure_delay",
			Help: "Average departure delay of all trains",
		},
		[]string{
			"station",
		},
	)
	stationArrivalDelay = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "wheresalice",
			Subsystem: "rtt",
			Name: "average_train_arrival_delay",
			Help: "Average arrival delay of all trains",
		},
		[]string{
			"station",
		},
	)
)

var config struct {
	RttUsername string
	RttPassword string
}

func main() {
	config.RttUsername = os.Getenv("RTT_USERNAME")
	config.RttPassword = os.Getenv("RTT_PASSWORD")

	prometheus.MustRegister(stationDepartureDelay)
	prometheus.MustRegister(stationArrivalDelay)

	go metricsHandler()

	metricsUpdate()
}

func metricsHandler() {
	flag.Parse()
	http.Handle("/metrics", prometheus.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func getRTT(station string) rtt.Station {
	log.Printf("getting rtt data for %s", station)

	var stationData rtt.Station
	rtt_url := fmt.Sprintf("https://%s:%s@api.rtt.io/api/v1/json/search/%s", config.RttUsername, config.RttPassword, station)
	response, _ := http.Get(rtt_url)
	buf, _ := ioutil.ReadAll(response.Body)
	stationData, err := rtt.UnmarshalStation(buf)
	if err != nil {
		log.Fatalf("failed parsing station data: %v", err)
	}
	return stationData
}

func averageDepartureDelay(station rtt.Station) int {
	service := station.Services
	var totalDelay int
	for _, v := range service {
		realDeparture, _ := strconv.Atoi(v.LocationDetail.RealtimeDeparture)
		bookedDeparture, _ := strconv.Atoi(v.LocationDetail.GbttBookedDeparture)
		log.Printf("scheduled: %v, actual: %v", bookedDeparture, realDeparture)
		departureDelay := realDeparture - bookedDeparture
		totalDelay += departureDelay
	}
	averageDelay := totalDelay / len(service)
	log.Printf("average departure delay for %s: %v", station.Location.Name, averageDelay)
	return averageDelay
}


func averageArrivalDelay(station rtt.Station) int {
	service := station.Services
	var totalDelay int
	var serviceCount int
	serviceCount = 0
	for _, v := range service {
		if v.LocationDetail.RealtimeArrival == nil || v.LocationDetail.GbttBookedArrival == nil {

		} else {
			realArrival, _ := strconv.Atoi(*v.LocationDetail.RealtimeArrival)
			bookedArrival, _ := strconv.Atoi(*v.LocationDetail.GbttBookedArrival)
			log.Printf("scheduled: %v, actual: %v", bookedArrival, realArrival)
			arrivalDelay := realArrival - bookedArrival
			totalDelay += arrivalDelay
			serviceCount ++
		}
	}
	averageDelay := totalDelay / serviceCount
	log.Printf("average arrival delay for %s: %v", station.Location.Name, averageDelay)
	return averageDelay
}

func metricsUpdate() {
	LDS := getRTT("LDS")
	departureDelay := averageDepartureDelay(LDS)
	arrivalDelay := averageArrivalDelay(LDS)

	stationArrivalDelay.With(prometheus.Labels{"station": "LDS"}).Set(float64(arrivalDelay))
	stationDepartureDelay.With(prometheus.Labels{"station": "LDS"}).Set(float64(departureDelay))

	for x := range time.Tick(5*time.Minute) {
		log.Printf("%v: updating metrics", x)

		LDS := getRTT("LDS")
		departureDelay := averageDepartureDelay(LDS)
		arrivalDelay := averageArrivalDelay(LDS)

		stationDepartureDelay.With(prometheus.Labels{"station": "LDS"}).Set(float64(departureDelay))
		stationArrivalDelay.With(prometheus.Labels{"station": "LDS"}).Set(float64(arrivalDelay))
	}
}