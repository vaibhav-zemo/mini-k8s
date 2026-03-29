package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"mini-k8s/pkg/models"
	"net/http"
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
	createNode("node-1", 4, 8192)
	createNode("node-2", 4, 8192)

	mux := http.NewServeMux()

	mux.HandleFunc("/pods", podsHandler)
	mux.HandleFunc("/pods/", podHandler)

	mux.HandleFunc("/nodes", nodesHandler)
	mux.HandleFunc("/nodes/", nodeHandler)

	mux.HandleFunc("/pods-by-node", podsByNodeHandler)

	mux.HandleFunc("/deployments", deploymentsHandler)
	mux.HandleFunc("/deployments/", deploymentHandler)

	mux.HandleFunc("/services", servicesHandler)
	mux.HandleFunc("/watch", watchHandler)

	log.Println("API Server running on :8080")
	http.ListenAndServe(":8080", mux)
}

func createNode(name string, cpu int, memory int) {
	clusterState.Nodes[name] = &models.Node{
		Name:        name,
		TotalCPU:    cpu,
		UsedCPU:     0,
		TotalMemory: memory,
		UsedMemory:  0,
	}
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

func nodesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(clusterState.Nodes)
	}
}

func nodeHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/nodes/")

	node, exists := clusterState.Nodes[name]
	if !exists {
		http.Error(w, "Node not found", 404)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var updated models.Node
		json.NewDecoder(r.Body).Decode(&updated)

		node.UsedCPU = updated.UsedCPU
		node.UsedMemory = updated.UsedMemory

		broadcast(models.Event{
			Type: "UPDATED",
			Kind: "NODE",
			Data: node,
		})

	case http.MethodDelete:
		var deleted models.Node
		json.NewDecoder(r.Body).Decode(&deleted)

		delete(clusterState.Nodes, name)
		broadcast(models.Event{
			Type: "DELETED",
			Kind: "NODE",
			Data: deleted,
		})
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
		pod.NodeName = updated.NodeName

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
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(clusterState.Deployments)
	case http.MethodPost:
		var d models.Deployment
		json.NewDecoder(r.Body).Decode(&d)

		d.ID = uuid.New().String()
		clusterState.Deployments[d.ID] = &d

		broadcast(models.Event{
			Type: "ADDED",
			Kind: "DEPLOYMENT",
			Data: d,
		})

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
