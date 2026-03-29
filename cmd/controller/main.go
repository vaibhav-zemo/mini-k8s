package main

import (
	"bufio"
	"encoding/json"
	"log"
	"mini-k8s/pkg/clients"
	"mini-k8s/pkg/models"
	"mini-k8s/pkg/utils"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

const apiServer = "http://localhost:8080"

var podCache = make(map[string]*models.Pod)
var nodeCache = make(map[string]*models.Node)
var deploymentCache = make(map[string]*models.Deployment)

func main() {
	initialSync()
	go startWatcher()

	reconcileLoop()
}

func reconcileLoop() {
	for {
		for _, d := range deploymentCache {
			count := 0
			for _, pod := range podCache {
				if pod.DeploymentID == d.ID {
					count++
				}
			}

			// SCALE UP
			if count < d.Replicas {
				toCreate := d.Replicas - count
				for i := 0; i < toCreate; i++ {
					createPod(d)
				}
			}

			// SCALE DOWN
			if count > d.Replicas {
				toDelete := count - d.Replicas
				for id, pod := range podCache {
					if pod.DeploymentID == d.ID && toDelete > 0 {
						deletePod(id, pod)
						toDelete--
					}
				}
			}
		}

		handleFailedPods()

		time.Sleep(2 * time.Second)
	}
}

func handleFailedPods() {
	for _, pod := range podCache {
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
					releaseNodeUsage(pod)
				}

			case "Never":
				log.Printf("[Controller] Pod %s will not restart", pod.ID)
				releaseNodeUsage(pod)
			}
		}
	}
}

func releaseNodeUsage(pod *models.Pod) {
	node := nodeCache[pod.NodeName]
	node.UsedCPU -= pod.CPU
	node.UsedMemory -= pod.Memory
	clients.UpdateNode(node)
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

func deletePod(id string, pod *models.Pod) {
	if pod.ContainerID != "" {
		err := stopAndRemoveContainer(pod.ContainerID)
		if err != nil {
			log.Printf("Failed to cleanup container for pod %s\n", id)
		}
	}

	delete(podCache, id)
	clients.DeletePod(pod)
	log.Printf("[ReplicaController] Deleted pod %s\n", id)
}

func createPod(deployment *models.Deployment) {
	pod := &models.Pod{
		ID:            uuid.New().String(),
		Image:         deployment.Image,
		Status:        "Pending",
		DeploymentID:  deployment.ID,
		RestartPolicy: "Always",
		CPU:           1,
		Memory:        512,
	}

	podCache[pod.ID] = pod
	clients.CreatePod(pod)

	log.Printf("[ReplicaController] Created pod %s for deployment %s\n", pod.ID, deployment.ID)
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

	case "DEPLOYMENT":
		deployment := &models.Deployment{}
		utils.Convert(event.Data, deployment)
		if event.Type == "DELETED" {
			delete(podCache, deployment.ID)
		} else {
			deploymentCache[deployment.ID] = deployment
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

	// deployments
	deployments := clients.GetDeployments()
	for _, d := range deployments {
		deploymentCache[d.ID] = d
	}

	log.Println("Initial sync done")
}
