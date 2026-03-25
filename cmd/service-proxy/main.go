package main

import (
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

func main() {
	rand.Seed(time.Now().UnixNano())

	mux := http.NewServeMux()
	mux.HandleFunc("/service/", serviceProxyHandler)

	log.Println("Service Proxy running on :9090")
	http.ListenAndServe(":9090", mux)
}

func fetchServiceByName(name string) *models.Service {
	resp, err := http.Get(apiServer + "/services")
	if err != nil {
		return nil
	}

	var services map[string]*models.Service
	json.NewDecoder(resp.Body).Decode(&services)

	for _, s := range services {
		if s.Name == name {
			return s
		}
	}

	return nil
}

func fetchPods() []*models.Pod {
	resp, err := http.Get(apiServer + "/pods")
	if err != nil {
		return nil
	}

	var podsMap map[string]*models.Pod
	json.NewDecoder(resp.Body).Decode(&podsMap)

	var pods []*models.Pod
	for _, p := range podsMap {
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
