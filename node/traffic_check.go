package node

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	usageFile = filepath.Join(os.TempDir(), "traffic_usage.dat")
	monthFile = filepath.Join(os.TempDir(), "traffic_cycle_month.dat")
)

func getEnvInt(key string, defaultValue int64) int64 {
	valStr := os.Getenv(key)
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return defaultValue
	}
	return val
}

func getCurrentTrafficBytes() (int64, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var total int64
	scanner := bufio.NewScanner(file)
	for i := 0; scanner.Scan(); i++ {
		if i < 2 {
			continue // skip headers
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		recv, _ := strconv.ParseInt(fields[1], 10, 64)
		send, _ := strconv.ParseInt(fields[9], 10, 64)
		total += recv + send
	}
	return total, nil
}

func readFileInt(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	val, _ := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	return val
}

func writeFileInt(path string, val int64) {
	buf := make([]byte, 0, 32)
	buf = fmt.Appendf(buf, "%d", val)
	err := os.WriteFile(path, buf, 0644)
	if err != nil {
		log.Printf("[!] Failed to write %d to %s: %v", val, path, err)
	}
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func blockAllExcept22() {
	log.Println("[!] Blocking all ports except 22...")
	runCommand("iptables", "-F")
	runCommand("iptables", "-P", "INPUT", "DROP")
	runCommand("iptables", "-A", "INPUT", "-p", "tcp", "--dport", "22", "-j", "ACCEPT")
	runCommand("iptables", "-A", "INPUT", "-i", "lo", "-j", "ACCEPT")
	runCommand("iptables", "-A", "INPUT", "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT")
}

func RestoreFirewall() {
	log.Println("[*] Restoring all firewall rules...")
	runCommand("iptables", "-F")
	runCommand("iptables", "-P", "INPUT", "ACCEPT")
}

// CheckTriffic checks the traffic usage of host and blocks all except port 22 if usage exceeds the limit.
func CheckTriffic() {
	// read environment variables
	trafficLimitGB := getEnvInt("TRAFFIC_LIMIT_GB", 10) // default 10GB/month
	resetDay := getEnvInt("CYCLE_RESET_DAY", 1)         // default data resets on the 1st day of the month

	curtime := time.Now()
	curDay := curtime.Day()
	curMonth := int(curtime.Month()) + curtime.Year()*100 // e.g., 202406

	// check if enters a new cycle
	lastMonth := readFileInt(monthFile)
	if int(curDay) == int(resetDay) && int(lastMonth) != curMonth {
		log.Println("[*] Monthly reset triggered.")
		RestoreFirewall()
		writeFileInt(usageFile, 0)               // reset traffic usage
		writeFileInt(monthFile, int64(curMonth)) // update current month
		return
	}

	// get current traffic usage
	curTraffic, err := getCurrentTrafficBytes()
	if err != nil {
		log.Fatal(err)
	}
	startTraffic := readFileInt(usageFile)
	if startTraffic == -1 || curTraffic < startTraffic {
		writeFileInt(usageFile, curTraffic)
		writeFileInt(monthFile, int64(curMonth))
		startTraffic = curTraffic
		log.Println("[*] Initial traffic usage recorded. " + fmt.Sprintf("%d %d bytes", startTraffic, curTraffic))
	}

	// calculate traffic usage
	usedBytes := curTraffic - startTraffic
	usedGB := usedBytes / (1024 * 1024 * 1024)
	percent := (usedGB * 100) / trafficLimitGB

	log.Printf("[*] Used traffic: %dGB / %dGB (%d%%)\n", usedGB, trafficLimitGB, percent)

	if percent >= 95 {
		blockAllExcept22()
	}
}
