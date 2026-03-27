package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mini-k8s/pkg/models"
	"net/http"
	"strings"
	"time"
)

const apiServer = "http://localhost:8080"

var podCache = make(map[string]*models.Pod)
var serviceCache = make(map[string]*models.Service)

func main() {
	rand.Seed(time.Now().UnixNano())

	initialSync()
	go startWatcher()

	mux := http.NewServeMux()
	mux.HandleFunc("/service/", serviceProxyHandler)

	log.Println("Service Proxy running on :9090")
	http.ListenAndServe(":9090", mux)
}

func startWatcher() {
	for {
		watchLoop()
		log.Println("Reconnecting to watch...")
		time.Sleep(2 * time.Second)
	}
}

func handleEvent(event models.Event) {

	switch event.Kind {

	case "POD":
		pod := &models.Pod{}
		convert(event.Data, pod)

		if event.Type == "DELETED" {
			delete(podCache, pod.ID)
		} else {
			podCache[pod.ID] = pod
		}

	case "SERVICE":
		svc := &models.Service{}
		convert(event.Data, svc)

		if event.Type == "DELETED" {
			delete(serviceCache, svc.ID)
		} else {
			serviceCache[svc.ID] = svc
		}
	}
}

func convert(input interface{}, output interface{}) {
	b, _ := json.Marshal(input)
	json.Unmarshal(b, output)
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

func initialSync() {
	// pods
	resp, _ := http.Get(apiServer + "/pods")
	var pods map[string]*models.Pod
	json.NewDecoder(resp.Body).Decode(&pods)

	for _, p := range pods {
		podCache[p.ID] = p
	}

	// services
	resp2, _ := http.Get(apiServer + "/services")
	var services map[string]*models.Service
	json.NewDecoder(resp2.Body).Decode(&services)

	for _, s := range services {
		serviceCache[s.ID] = s
	}

	log.Println("Initial sync done")
}

func fetchServiceByName(name string) *models.Service {
	for _, s := range serviceCache {
		if s.Name == name {
			return s
		}
	}

	return nil
}

func fetchPods() []*models.Pod {

	var pods []*models.Pod
	for _, p := range podCache {
		pods = append(pods, p)
	}

	return pods
}

func filterPods(deploymentID string, pods []*models.Pod) []*models.Pod {
	var result []*models.Pod

	for _, pod := range pods {
		if pod.DeploymentID == deploymentID && pod.Status == "Running" {
			result = append(result, pod)
		}
	}

	return result
}

func pickPod(pods []*models.Pod) *models.Pod {
	return pods[rand.Intn(len(pods))]
}

func proxyToPod(w http.ResponseWriter, r *http.Request, pod *models.Pod) {

	url := fmt.Sprintf("http://localhost:%d", pod.Port)

	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, "Pod unreachable", 500)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func serviceProxyHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/service/")

	service := fetchServiceByName(name)
	if service == nil {
		http.Error(w, "Service not found", 404)
		return
	}

	pods := fetchPods()
	runningPods := filterPods(service.DeploymentID, pods)

	if len(runningPods) == 0 {
		http.Error(w, "No running pods", 503)
		return
	}

	pod := pickPod(runningPods)

	proxyToPod(w, r, pod)
}
