package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	//	"github.com/pborman/getopt/v2"    // BSD-3
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/spf13/viper"
	"time"
)

type Limits struct {
	Warning int `mapstructure:"Warning"`
	Error   int `mapstructure:"Error"`
}

type Service struct {
	Description string `mapstructure:"Description"`
	Port        string `mapstructure:"Port"`
}

var (
	configFileName = "ohdear-health"
	defaultSecret  = "set-secret-in-config-file"
)

func readConfig() {
	viper.SetConfigName(configFileName)
	viper.AddConfigPath(".")
	viper.SetDefault("Core.Listen", ":8991")
	viper.SetDefault("Core.Secret", defaultSecret)

	err := viper.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		log.Fatalf("Cannot read config file: %s\n", err)
	}

	healthCheckSecret := viper.GetString("Core.Secret")
	if healthCheckSecret == defaultSecret {
		log.Fatalf("You must set Core.Secret in the '%s.yaml' config file.\n", configFileName)
	}
}

func checkSecret(w http.ResponseWriter, r *http.Request) bool {
	secretFromHeader := r.Header["Oh-Dear-Health-Check-Secret"]

	if len(secretFromHeader) != 1 {
		return false
	}

	healthCheckSecret := viper.GetString("Core.Secret")

	if secretFromHeader[0] != healthCheckSecret {
		return false
	}

	return true
}

func getStatus(value float64, limits Limits) (string, string) {
	if value > float64(limits.Error) {
		return "failed", "is critical"
	}
	if value > float64(limits.Warning) {
		return "warning", "is high"
	}
	return "ok", "is fine"
}

type CheckResult struct {
	Name                string `json:"name"`
	Label               string `json:"label"`
	Status              string `json:"status"`
	NotificationMessage string `json:"notificationMessage"`
	ShortSummary        string `json:"shortSummary"`
}

type HealthCheckResult struct {
	FinishedAt   int64         `json:"finishedAt"`
	CheckResults []CheckResult `json:"checkResults"`
}

func CheckPartitions(checkResults *[]CheckResult, disk_usage_limits Limits) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		log.Fatal(err)
	}
	for i, partition := range partitions {
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			log.Fatal(err)
		}

		if partition.Fstype != "ext3" && partition.Fstype != "ext4" {
			continue
		}

		status, messageStatus := getStatus(usage.UsedPercent, disk_usage_limits)
		checkResult := CheckResult{
			Name:                fmt.Sprintf("UsedDiskSpace%d", i),
			Label:               fmt.Sprintf("Used Disk Space: %s", partition.Mountpoint),
			Status:              status,
			NotificationMessage: fmt.Sprintf("Disk usage %s (%.0f%% used)", messageStatus, usage.UsedPercent),
			ShortSummary:        fmt.Sprintf("%.0f%%", usage.UsedPercent),
		}
		*checkResults = append(*checkResults, checkResult)
	}
}

func CheckLoad(checkResults *[]CheckResult, load_avg_limits Limits) {
	loadAvg, err := load.Avg()
	if err != nil {
		log.Fatal(err)
	}

	status, messageStatus := getStatus(loadAvg.Load5, load_avg_limits)
	checkResult := CheckResult{
		Name:                "LoadAvg",
		Label:               "Load Average Over 5 Minutes",
		Status:              status,
		NotificationMessage: fmt.Sprintf("Load Average %s (%.1f)", messageStatus, loadAvg.Load5),
		ShortSummary:        fmt.Sprintf("%.1f", loadAvg.Load5),
	}
	*checkResults = append(*checkResults, checkResult)
}

func CheckMemory(checkResults *[]CheckResult, mem_usage_limits Limits) {
	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		log.Fatal(err)
	}

	status, messageStatus := getStatus(virtualMemory.UsedPercent, mem_usage_limits)
	checkResult := CheckResult{
		Name:                "MemUsage",
		Label:               "Memory Usage in Percentage",
		Status:              status,
		NotificationMessage: fmt.Sprintf("Memory Usage %s (%.1f%%)", messageStatus, virtualMemory.UsedPercent),
		ShortSummary:        fmt.Sprintf("%.1f%%", virtualMemory.UsedPercent),
	}
	*checkResults = append(*checkResults, checkResult)
}

func CheckTcpServices(checkResults *[]CheckResult, tcp_services []Service) {
	for _, service := range tcp_services {
		timeout := time.Second
		conn, err := net.DialTimeout("tcp", service.Port, timeout)
		if err != nil {
			checkResult := CheckResult{
				Name:                service.Description,
				Label:               fmt.Sprintf("%s Service Availability", service.Description),
				Status:              "failed",
				NotificationMessage: fmt.Sprintf("Service %s is not connectable", service.Description),
				ShortSummary:        "Can't Connect",
			}
			*checkResults = append(*checkResults, checkResult)
		}
		if conn != nil {
			defer conn.Close()

			checkResult := CheckResult{
				Name:                service.Description,
				Label:               fmt.Sprintf("%s Service Availability", service.Description),
				Status:              "ok",
				NotificationMessage: fmt.Sprintf("Service %s is available", service.Description),
				ShortSummary:        "Available",
			}
			*checkResults = append(*checkResults, checkResult)
		}
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	if !checkSecret(w, r) {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json")
		resp := make(map[string]string)
		resp["message"] = "Forbidden"
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Fatalf("Error happened in JSON marshal. Err: %s", err)
		}
		w.Write(jsonResp)

		return
	}

	// Re-read config
	err := viper.ReadInConfig()
	var load_avg_limits Limits
	var mem_usage_limits Limits
	var disk_usage_limits Limits
	var tcp_services []Service
	viper.UnmarshalKey("LoadAverage", &load_avg_limits)
	viper.UnmarshalKey("MemoryUsagePercent", &mem_usage_limits)
	viper.UnmarshalKey("DiskUsagePercent", &disk_usage_limits)
	viper.UnmarshalKey("TCPServices", &tcp_services)

	var checkResults []CheckResult

	CheckPartitions(&checkResults, disk_usage_limits)
	CheckLoad(&checkResults, load_avg_limits)
	CheckMemory(&checkResults, disk_usage_limits)
	CheckTcpServices(&checkResults, tcp_services)

	report := HealthCheckResult{FinishedAt: time.Now().Unix(), CheckResults: checkResults}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	jsonResp, err := json.MarshalIndent(report, "", "\t")

	if err != nil {
		log.Fatalf("Error happened in JSON marshal. Err: %s", err)
	}
	w.Write(jsonResp)

	return
}

func main() {
	readConfig()

	http.HandleFunc("/health-check", handler)
	log.Fatal(http.ListenAndServe(viper.GetString("Core.Listen"), nil))
}
