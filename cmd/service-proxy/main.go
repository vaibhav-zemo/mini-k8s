package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"mini-k8s/pkg/clients"
	"mini-k8s/pkg/models"
	"mini-k8s/pkg/utils"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const apiServer = "http://localhost:8080"

var podCache = make(map[string]*models.Pod)
var serviceCache = make(map[string]*models.Service)

func main() {
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
		utils.Convert(event.Data, pod)

		if event.Type == "DELETED" {
			delete(podCache, pod.ID)
		} else {
			podCache[pod.ID] = pod
		}

	case "SERVICE":
		svc := &models.Service{}
		utils.Convert(event.Data, svc)

		if event.Type == "DELETED" {
			delete(serviceCache, svc.ID)
		} else {
			serviceCache[svc.Name] = svc
		}
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

func initialSync() {
	// pods
	pods := clients.GetPods()
	for _, p := range pods {
		podCache[p.ID] = p
	}

	// services
	services := clients.GetServices()
	for _, s := range services {
		serviceCache[s.Name] = s
	}

	log.Println("Initial sync done")
}

func getPods() []*models.Pod {

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

func pickPodRR(service *models.Service, pods []*models.Pod) *models.Pod {
	if len(pods) == 0 {
		return nil
	}

	pod := pods[service.Index%len(pods)]
	service.Index = (service.Index + 1) % len(pods)

	return pod
}

func proxyToPod(w http.ResponseWriter, r *http.Request, pod *models.Pod) {
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", pod.Port))

	// preserve original request path
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {

			// set target (like Director used to do)
			pr.SetURL(target)

			// 🔥 FIX: strip "/service/{name}"
			parts := strings.SplitN(pr.In.URL.Path, "/", 4)
			if len(parts) >= 4 {
				pr.Out.URL.Path = "/" + parts[3]
			} else {
				pr.Out.URL.Path = "/"
			}

			// preserve query params
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
		},
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Pod unreachable", 502)
	}

	proxy.ServeHTTP(w, r)
}

func serviceProxyHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/service/")

	service := serviceCache[name]
	if service == nil {
		http.Error(w, "Service not found", 404)
		return
	}

	pods := getPods()
	runningPods := filterPods(service.DeploymentID, pods)

	if len(runningPods) == 0 {
		http.Error(w, "No running pods", 503)
		return
	}

	//pod := pickPod(runningPods)
	pod := pickPodRR(service, runningPods)

	proxyToPod(w, r, pod)
}
