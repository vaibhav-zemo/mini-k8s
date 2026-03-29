package clients

import (
	"bytes"
	"encoding/json"
	"mini-k8s/pkg/models"
	"net/http"
)

const apiServer = "http://localhost:8080"

func UpdatePod(pod *models.Pod) {
	data, _ := json.Marshal(pod)

	req, _ := http.NewRequest("PUT",
		apiServer+"/pods/"+pod.ID,
		bytes.NewBuffer(data),
	)

	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}

func UpdateNode(node *models.Node) {
	data, _ := json.Marshal(node)

	req, _ := http.NewRequest("PUT", apiServer+"/nodes/"+node.Name, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	http.DefaultClient.Do(req)
}

func CreatePod(pod *models.Pod) {
	data, _ := json.Marshal(pod)

	req, _ := http.NewRequest("POST", apiServer+"/pods", bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")

	http.DefaultClient.Do(req)
}

func DeletePod(pod *models.Pod) {
	req, _ := http.NewRequest(http.MethodDelete, apiServer+"/pods/"+pod.ID, nil)
	req.Header.Set("Content-Type", "application/json")

	http.DefaultClient.Do(req)
}

func GetPods() map[string]*models.Pod {
	resp, _ := http.Get(apiServer + "/pods")
	var pods map[string]*models.Pod
	json.NewDecoder(resp.Body).Decode(&pods)

	return pods
}

func GetNodes() map[string]*models.Node {
	resp, _ := http.Get(apiServer + "/nodes")
	var nodes map[string]*models.Node
	json.NewDecoder(resp.Body).Decode(&nodes)

	return nodes
}

func GetServices() map[string]*models.Service {
	resp, _ := http.Get(apiServer + "/services")
	var services map[string]*models.Service
	json.NewDecoder(resp.Body).Decode(&services)

	return services
}

func GetDeployments() map[string]*models.Deployment {
	resp, _ := http.Get(apiServer + "/deployments")
	var deployments map[string]*models.Deployment
	json.NewDecoder(resp.Body).Decode(&deployments)
	return deployments
}
