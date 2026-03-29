package main

import (
	"bufio"
	"encoding/json"
	"log"
	"mini-k8s/pkg/clients"
	"mini-k8s/pkg/models"
	"mini-k8s/pkg/utils"
	"net/http"
	"strings"
	"time"
)

const apiServer = "http://localhost:8080"

var podCache = make(map[string]*models.Pod)
var nodeCache = make(map[string]*models.Node)

func main() {
	initialSync()
	go startWatcher()

	scheduleLoop()
}

func schedulePod(pod *models.Pod) string {
	log.Printf("[Scheduler] Trying to schedule pod %s (CPU=%d, MEM=%d)", pod.ID, pod.CPU, pod.Memory)

	var selectedNode *models.Node
	bestScore := int(^uint(0) >> 1) // max int

	for _, node := range nodeCache {

		freeCPU := node.TotalCPU - node.UsedCPU
		freeMem := node.TotalMemory - node.UsedMemory

		// skip if not enough resources
		if freeCPU < pod.CPU || freeMem < pod.Memory {
			continue
		}

		// score = leftover resources (lower is better)
		score := (freeCPU - pod.CPU) + (freeMem - pod.Memory)

		if score < bestScore {
			bestScore = score
			selectedNode = node
		}
	}

	if selectedNode == nil {
		log.Printf("[Scheduler] No suitable node for pod %s", pod.ID)
		return "" // no suitable node
	}

	return selectedNode.Name
}

func scheduleLoop() {
	for {
		for _, pod := range podCache {

			if pod.Status == "Pending" {

				nodeName := schedulePod(pod)
				if nodeName == "" {
					continue
				}

				pod.NodeName = nodeName
				pod.Status = "Scheduled"
				clients.UpdatePod(pod)

				// update node stats
				node := nodeCache[nodeName]
				node.UsedCPU += pod.CPU
				node.UsedMemory += pod.Memory
				clients.UpdateNode(node)

				log.Printf("[Scheduler] Pod %s → %s", pod.ID, nodeName)
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

	// nodes
	nodes := clients.GetNodes()
	for _, n := range nodes {
		nodeCache[n.Name] = n
	}

	log.Println("Initial sync done")
}
