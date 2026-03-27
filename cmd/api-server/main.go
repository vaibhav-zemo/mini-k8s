package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"mini-k8s/pkg/models"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

var clusterState = struct {
	Pods        map[string]*models.Pod
	Nodes       map[string]*models.Node
	Deployments map[string]*models.Deployment
	Services    map[string]*models.Service
}{
	Pods:        make(map[string]*models.Pod),
	Nodes:       make(map[string]*models.Node),
	Deployments: make(map[string]*models.Deployment),
	Services:    make(map[string]*models.Service),
}

var subscribers = make([]chan models.Event, 0)

func main() {
	rand.Seed(time.Now().UnixNano())

	// init nodes
	clusterState.Nodes["node-1"] = &models.Node{Name: "node-1"}
	clusterState.Nodes["node-2"] = &models.Node{Name: "node-2"}

	mux := http.NewServeMux()

	mux.HandleFunc("/pods", podsHandler)
	mux.HandleFunc("/pods/", podHandler)
	mux.HandleFunc("/pods-by-node", podsByNodeHandler)
	mux.HandleFunc("/deployments", deploymentsHandler)
	mux.HandleFunc("/deployments/", deploymentHandler)
	mux.HandleFunc("/services", servicesHandler)
	mux.HandleFunc("/watch", watchHandler)

	go controllerLoop()

	log.Println("API Server running on :8080")
	http.ListenAndServe(":8080", mux)
}

func removeSubscriber(target chan models.Event) {
	for i, sub := range subscribers {
		if sub == target {
			subscribers = append(subscribers[:i], subscribers[i+1:]...)
			break
		}
	}
}

func broadcast(event models.Event) {
	for _, sub := range subscribers {
		select {
		case sub <- event:
		default:
			// skip slow subscriber
		}
	}
}

func watchHandler(w http.ResponseWriter, r *http.Request) {

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ch := make(chan models.Event, 10)

	// register subscriber
	subscribers = append(subscribers, ch)

	log.Println("New watcher connected")

	// detect disconnect
	notify := r.Context().Done()

	for {
		select {
		case event := <-ch:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-notify:
			log.Println("Watcher disconnected")
			removeSubscriber(ch)
			return
		}
	}
}

func servicesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodPost:
		var s models.Service
		err := json.NewDecoder(r.Body).Decode(&s)
		if err != nil {
			http.Error(w, "Invalid body", 400)
			return
		}

		s.ID = uuid.New().String()
		clusterState.Services[s.ID] = &s

		log.Printf("[API] Service created: %s\n", s.Name)

		broadcast(models.Event{
			Type: "ADDED",
			Kind: "SERVICE",
			Data: s,
		})

		json.NewEncoder(w).Encode(s)

	case http.MethodGet:
		json.NewEncoder(w).Encode(clusterState.Services)

	default:
		http.Error(w, "Method not allowed", 405)
	}
}

func podsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodPost:
		var pod models.Pod
		json.NewDecoder(r.Body).Decode(&pod)

		pod.ID = uuid.New().String()
		pod.Status = "Pending"

		if pod.RestartPolicy == "" {
			pod.RestartPolicy = "Always"
		}

		clusterState.Pods[pod.ID] = &pod

		broadcast(models.Event{
			Type: "ADDED",
			Kind: "POD",
			Data: pod,
		})

		json.NewEncoder(w).Encode(pod)

	case http.MethodGet:
		json.NewEncoder(w).Encode(clusterState.Pods)
	}
}

func podHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/pods/")

	pod, exists := clusterState.Pods[id]
	if !exists {
		http.Error(w, "Not found", 404)
		return
	}

	switch r.Method {

	case http.MethodGet:
		json.NewEncoder(w).Encode(pod)

	case http.MethodDelete:
		broadcast(models.Event{
			Type: "DELETED",
			Kind: "POD",
			Data: pod,
		})
		delete(clusterState.Pods, id)

	case http.MethodPut:
		var updated models.Pod
		json.NewDecoder(r.Body).Decode(&updated)

		pod.Status = updated.Status
		pod.ContainerID = updated.ContainerID
		pod.RetryCount = updated.RetryCount
		pod.Port = updated.Port

		broadcast(models.Event{
			Type: "UPDATED",
			Kind: "POD",
			Data: pod,
		})
	}
}

func podsByNodeHandler(w http.ResponseWriter, r *http.Request) {
	node := r.URL.Query().Get("node")

	var result []*models.Pod

	for _, pod := range clusterState.Pods {
		if pod.NodeName == node {
			result = append(result, pod)
		}
	}

	json.NewEncoder(w).Encode(result)
}

func deploymentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var d models.Deployment
		json.NewDecoder(r.Body).Decode(&d)

		d.ID = uuid.New().String()
		clusterState.Deployments[d.ID] = &d

		json.NewEncoder(w).Encode(d)
	}
}

func deploymentHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/deployments/")

	d, exists := clusterState.Deployments[id]
	if !exists {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	switch r.Method {

	case http.MethodGet:
		json.NewEncoder(w).Encode(d)

	case http.MethodPut:
		var updated models.Deployment
		err := json.NewDecoder(r.Body).Decode(&updated)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Only update replicas (important design decision)
		if updated.Replicas >= 0 {
			log.Printf("[API] Scaling deployment %s from %d → %d\n",
				id, d.Replicas, updated.Replicas)

			d.Replicas = updated.Replicas
		}

		json.NewEncoder(w).Encode(d)

	case http.MethodDelete:
		delete(clusterState.Deployments, id)
		log.Printf("[API] Deleted deployment %s\n", id)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ---------- CONTROLLER ----------

func controllerLoop() {
	for {

		// -------------------------------
		// 1. REPLICA CONTROLLER
		// -------------------------------
		for _, d := range clusterState.Deployments {

			count := 0

			// count current pods for this deployment
			for _, pod := range clusterState.Pods {
				if pod.DeploymentID == d.ID {
					count++
				}
			}

			// SCALE UP
			if count < d.Replicas {
				toCreate := d.Replicas - count

				for i := 0; i < toCreate; i++ {
					pod := &models.Pod{
						ID:            uuid.New().String(),
						Image:         d.Image,
						Status:        "Pending",
						DeploymentID:  d.ID,
						RestartPolicy: "Always",
					}

					clusterState.Pods[pod.ID] = pod
					broadcast(models.Event{
						Type: "ADDED",
						Kind: "POD",
						Data: pod,
					})
					log.Printf("[ReplicaController] Created pod %s for deployment %s\n", pod.ID, d.ID)
				}
			}

			// SCALE DOWN
			if count > d.Replicas {
				toDelete := count - d.Replicas

				for id, pod := range clusterState.Pods {
					if pod.DeploymentID == d.ID && toDelete > 0 {

						if pod.ContainerID != "" {
							err := stopAndRemoveContainer(pod.ContainerID)
							if err != nil {
								log.Printf("Failed to cleanup container for pod %s\n", id)
							}
						}

						delete(clusterState.Pods, id)
						toDelete--

						broadcast(models.Event{
							Type: "DELETED",
							Kind: "POD",
							Data: pod,
						})
						log.Printf("[ReplicaController] Deleted pod %s\n", id)
					}
				}
			}
		}

		// -------------------------------
		// 2. SCHEDULER (Pending → Scheduled)
		// -------------------------------
		for _, pod := range clusterState.Pods {

			if pod.Status == "Pending" {

				node := schedulePod()
				if node == "" {
					continue
				}

				pod.NodeName = node
				pod.Status = "Scheduled"

				log.Printf("[Scheduler] Pod %s scheduled on %s\n", pod.ID, node)
			}
		}

		// -------------------------------
		// 3. FAILURE HANDLING (Retry logic)
		// -------------------------------
		for _, pod := range clusterState.Pods {

			if pod.Status == "Failed" {

				switch pod.RestartPolicy {

				case "Always":
					log.Printf("[Controller] Retrying pod %s (Always)", pod.ID)
					pod.Status = "Scheduled"

				case "OnFailure":
					if pod.RetryCount < 3 {
						log.Printf("[Controller] Retrying pod %s (OnFailure)", pod.ID)
						pod.Status = "Scheduled"
					} else {
						log.Printf("[Controller] Pod %s exceeded retry limit", pod.ID)
					}

				case "Never":
					log.Printf("[Controller] Pod %s will not restart", pod.ID)
				}
			}
		}

		// -------------------------------
		// LOOP INTERVAL
		// -------------------------------
		time.Sleep(2 * time.Second)
	}
}

func stopAndRemoveContainer(containerID string) error {
	if containerID == "" {
		return nil
	}

	// Stop container
	stopCmd := exec.Command("docker", "stop", containerID)
	stopOut, err := stopCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error stopping container %s: %s\n", containerID, string(stopOut))
		return err
	}

	// Remove container (important to avoid clutter)
	rmCmd := exec.Command("docker", "rm", containerID)
	rmOut, err := rmCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error removing container %s: %s\n", containerID, string(rmOut))
		return err
	}

	log.Printf("Container %s stopped and removed\n", containerID)
	return nil
}

func schedulePod() string {
	for _, node := range clusterState.Nodes {
		return node.Name
	}
	return ""
}
