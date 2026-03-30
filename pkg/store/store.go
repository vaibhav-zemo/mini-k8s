package store

import (
	"encoding/json"
	"log"
	"mini-k8s/pkg/models"

	"go.etcd.io/bbolt"
)

type BoltStore struct {
	db *bbolt.DB
}

var (
	podBucket        = []byte("pods")
	deploymentBucket = []byte("deployments")
	serviceBucket    = []byte("services")
	nodeBucket       = []byte("nodes")
)

func NewBoltStore(path string) *BoltStore {
	db, err := bbolt.Open(path, 0666, nil)
	if err != nil {
		log.Fatal(err)
	}

	// create buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		_, _ = tx.CreateBucketIfNotExists(podBucket)
		_, _ = tx.CreateBucketIfNotExists(deploymentBucket)
		_, _ = tx.CreateBucketIfNotExists(serviceBucket)
		_, _ = tx.CreateBucketIfNotExists(nodeBucket)
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	return &BoltStore{db: db}
}

func (s *BoltStore) SavePod(pod *models.Pod) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(podBucket)

		data, _ := json.Marshal(pod)
		return b.Put([]byte(pod.ID), data)
	})
}

func (s *BoltStore) SaveDeployment(deployment *models.Deployment) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(deploymentBucket)
		data, _ := json.Marshal(deployment)
		return b.Put([]byte(deployment.ID), data)
	})
}

func (s *BoltStore) SaveService(service *models.Service) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(serviceBucket)
		data, _ := json.Marshal(service)
		return b.Put([]byte(service.ID), data)
	})
}

func (s *BoltStore) SaveNode(node *models.Node) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(nodeBucket)
		data, _ := json.Marshal(node)
		return b.Put([]byte(node.Name), data)
	})
}

func (s *BoltStore) GetPods() ([]*models.Pod, error) {
	var pods []*models.Pod

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(podBucket)

		return b.ForEach(func(k, v []byte) error {
			var pod models.Pod
			json.Unmarshal(v, &pod)
			pods = append(pods, &pod)
			return nil
		})
	})

	return pods, err
}

func (s *BoltStore) GetDeployments() ([]*models.Deployment, error) {
	var deployments []*models.Deployment

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(deploymentBucket)
		return b.ForEach(func(k, v []byte) error {
			var deployment models.Deployment
			json.Unmarshal(v, &deployment)
			deployments = append(deployments, &deployment)
			return nil
		})
	})

	return deployments, err
}

func (s *BoltStore) GetServices() ([]*models.Service, error) {
	var services []*models.Service
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(serviceBucket)
		return b.ForEach(func(k, v []byte) error {
			var service models.Service
			json.Unmarshal(v, &service)
			services = append(services, &service)
			return nil
		})
	})

	return services, err
}

func (s *BoltStore) GetNodes() ([]*models.Node, error) {
	var nodes []*models.Node
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(nodeBucket)
		return b.ForEach(func(k, v []byte) error {
			var node models.Node
			json.Unmarshal(v, &node)
			nodes = append(nodes, &node)
			return nil
		})
	})

	return nodes, err
}

func (s *BoltStore) DeletePod(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(podBucket)
		return b.Delete([]byte(id))
	})
}
