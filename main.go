package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/joho/godotenv"
	"github.com/niluan304/ddcci"
)

type HAConfig struct {
	Address string
	Port    string
	Token   string
	Entity  string
}

type SensorResponse struct {
	Value        string `json:"state"`
	LastChanged  string `json:"last_changed"`
	LastUpdated  string `json:"last_updated"`
	LastReported string `json:"last_reported"`
}

type BrightnessRange struct {
	Min        int
	Max        int
	Percentage int
}

// based on https://learn.microsoft.com/en-us/windows-hardware/design/device-experiences/sensors-adaptive-brightness#ambient-light-response-curve-changes-for-windows-11
var ranges = []BrightnessRange{
	{Min: 0, Max: 10, Percentage: 10},
	{Min: 5, Max: 50, Percentage: 25},
	{Min: 15, Max: 100, Percentage: 40},
	{Min: 60, Max: 300, Percentage: 55},
	{Min: 150, Max: 400, Percentage: 70},
	{Min: 250, Max: 650, Percentage: 85},
	{Min: 350, Max: 2000, Percentage: 100},
	{Min: 1000, Max: 7000, Percentage: 115},
	{Min: 5000, Max: 10000, Percentage: 130},
}

func getConfig() HAConfig {
	return HAConfig{
		Address: os.Getenv("HA_ADDRESS"),
		Port:    os.Getenv("HA_PORT"),
		Entity:  os.Getenv("HA_ENTITY"),
		Token:   os.Getenv("HA_TOKEN"),
	}
}

func buildRequest(method string, url string, headers map[string]string) *http.Request {
	req, _ := http.NewRequest(method, url, strings.NewReader(""))
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	return req
}

func getSensor(config HAConfig) *SensorResponse {
	url := fmt.Sprintf("http://%v:%v/api/states/%v", config.Address, config.Port, config.Entity)
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %v", config.Token),
		"Content-Type":  "application/json",
	}
	req := buildRequest("GET", url, headers)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("error getting response: %v", err)
	}
	return readResponse(resp)
}

func readResponse(resp *http.Response) *SensorResponse {
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("error reading response: %v", err)
	}
	sensorResponse := SensorResponse{}
	json.Unmarshal(response, &sensorResponse)
	return &sensorResponse
}

func getMonitor(id int) *ddcci.PhysicalMonitor {
	monitors, err := ddcci.NewSystemMonitors()
	if err != nil {
		log.Errorf("Error getting monitors: %v", err)
	}
	m, err := ddcci.NewPhysicalMonitor(&monitors[id])
	if err != nil {
		log.Errorf("Error getting the specified monitor: %v", err)
	}
	return m
}

func getBrightness(monitor *ddcci.PhysicalMonitor) int {
	_, current, _, err := monitor.GetBrightness()
	if err != nil {
		log.Errorf("Error getting monitor brightness: %v", err)
	}
	return current
}

func getPercentage(value int) int {
	for _, r := range ranges {
		if value >= r.Min && value < r.Max {
			return r.Percentage
		}
	}
	return 0
}

func setBrightness(monitor *ddcci.PhysicalMonitor, value int) {
	log.Infof("setting monitor brightness to %v%%", value)
	err := monitor.SetBrightness(value)
	if err != nil {
		log.Errorf("Error setting monitor brightness: %v", err)
	}
}

func main() {
	godotenv.Load()
	config := getConfig()
	monitor := getMonitor(0)
	current := getBrightness(monitor)
	log.Infof("current brightness: %v", current)

	sensor := getSensor(config)
	log.Infof("sensor value: %v", sensor.Value)
	value, err := strconv.ParseFloat(sensor.Value, 64)
	if err != nil {
		log.Errorf("Error parsing sensor value: %v", err)
	}
	percentage := getPercentage(int(value))
	if percentage != current {
		setBrightness(monitor, percentage)
	} else {
		log.Warnf("brightness is already at %v%%, skipping...", current)
	}
}
