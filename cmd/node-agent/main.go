package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"mini-k8s/pkg/clients"
	"mini-k8s/pkg/models"
	"mini-k8s/pkg/utils"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

const apiServer = "http://localhost:8080"

var podCache = make(map[string]*models.Pod)
var nodeName = "node-1"

func main() {
	initialSync()
	go startWatcher()

	for {
		pods := podCache

		for _, pod := range pods {

			if pod.Status == "Scheduled" {
				runAndUpdate(pod)
			}

			if pod.Status == "Running" {
				checkAndRestart(pod)
			}
		}

		time.Sleep(2 * time.Second)
	}
}

func startWatcher() {
	for {
		watchLoop()
		log.Println("Reconnecting to watch...")
		time.Sleep(2 * time.Second)
	}
}

func watchLoop() {

	resp, err := http.Get(apiServer + "/watch")
	if err != nil {
		log.Println("Watch connection failed")
		return
	}

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Watch disconnected")
			return
		}

		if strings.HasPrefix(line, "data: ") {
			var event models.Event
			json.Unmarshal([]byte(line[6:]), &event)

			handleEvent(event)
		}
	}
}

func handleEvent(event models.Event) {

	if event.Kind != "POD" {
		return
	}

	pod := &models.Pod{}
	utils.Convert(event.Data, pod)

	if pod.NodeName != nodeName {
		return
	}

	if event.Type == "DELETED" {
		delete(podCache, pod.ID)
	} else {
		podCache[pod.ID] = pod
	}
}

func initialSync() {
	resp, _ := http.Get(apiServer + "/pods-by-node?node=" + nodeName)

	var pods map[string]*models.Pod
	json.NewDecoder(resp.Body).Decode(&pods)

	log.Println("Initial sync done")
}

func getFreePort() int {
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func runAndUpdate(pod *models.Pod) {

	port := getFreePort()

	cmd := exec.Command(
		"docker", "run", "-d",
		"-p", fmt.Sprintf("%d:80", port),
		pod.Image,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("Run failed:", string(out))

		pod.Status = "Failed"
		pod.RetryCount++
		clients.UpdatePod(pod)
		return
	}

	pod.ContainerID = strings.TrimSpace(string(out))
	pod.Port = port
	pod.Status = "Running"

	log.Printf("Started container %s on port %d\n", pod.ContainerID, port)

	clients.UpdatePod(pod)
}

func checkAndRestart(pod *models.Pod) {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", pod.ContainerID)

	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		log.Println("Container crashed:", pod.ContainerID)

		pod.Status = "Scheduled"
		clients.UpdatePod(pod)
	}
}
