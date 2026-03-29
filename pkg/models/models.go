package models

type Pod struct {
	ID            string `json:"id"`
	Image         string `json:"image"`
	Status        string `json:"status"`
	NodeName      string `json:"nodeName"`
	ContainerID   string `json:"containerId"`
	RestartPolicy string `json:"restartPolicy"`
	RetryCount    int    `json:"retryCount"`
	DeploymentID  string `json:"deploymentId"`
	Port          int    `json:"port"`
	CPU           int    `json:"cpu"`
	Memory        int    `json:"memory"`
}

type Node struct {
	Name        string `json:"name"`
	TotalCPU    int    `json:"totalCpu"`
	UsedCPU     int    `json:"usedCpu"`
	TotalMemory int    `json:"totalMemory"`
	UsedMemory  int    `json:"usedMemory"`
}

type Deployment struct {
	ID       string `json:"id"`
	Image    string `json:"image"`
	Replicas int    `json:"replicas"`
}

type Service struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DeploymentID string `json:"deploymentId"`
	Port         int    `json:"port"`
}

type Event struct {
	Type string      `json:"type"` // ADDED, UPDATED, DELETED
	Kind string      `json:"kind"` // POD, SERVICE
	Data interface{} `json:"data"`
}
