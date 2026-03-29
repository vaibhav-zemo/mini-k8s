# 🚀 Mini Kubernetes in Go

A simplified Kubernetes-like container orchestration system built from scratch in Go to understand core distributed systems concepts like scheduling, reconciliation, service discovery, and event-driven architecture.

---

## 🧠 Overview

This project simulates the core architecture of Kubernetes by splitting responsibilities across multiple components:

* **API Server (Control Plane)** – Stores cluster state and exposes APIs
* **Scheduler** – Assigns pods to nodes based on resource availability
* **Controller** – Ensures desired state matches actual state (reconciliation loop)
* **Node Agent (Worker)** – Runs containers using Docker and maintains pod lifecycle
* **Service Proxy (Data Plane)** – Provides load balancing across pods
* **Watch API** – Event-driven updates using streaming (LIST + WATCH pattern)

---

## ⚙️ Architecture

```
                +------------------+
                |   API Server     |
                | (State + Watch)  |
                +------------------+
                         |
        -------------------------------------
        |            |            |          |
 +-------------+ +-------------+ +-----------+
 | Scheduler   | | Controller  | | Proxy     |
 +-------------+ +-------------+ +-----------+
                         |
                 +----------------+
                 |  Node Agent    |
                 +----------------+
```

---

## ✨ Features

### 🧩 Core Orchestration

* Pod lifecycle management (Pending → Scheduled → Running)
* Deployment support with replica management
* Self-healing (restart failed containers)
* Desired state reconciliation

### ⚡ Scheduling

* Resource-aware scheduling (CPU + Memory)
* Best-fit node selection
* Handles insufficient resource scenarios

### 🔄 Event-Driven System

* LIST + WATCH pattern (inspired by Kubernetes)
* Streaming updates via HTTP (SSE)
* Local caching in services (no polling)

### 🌐 Service & Networking

* Service abstraction over deployments
* Load balancing across pods
* Randomized request routing

### 🐳 Container Management

* Docker-based container execution
* Dynamic port allocation
* Container health checking & restart

### 🧠 Distributed Design

* Separation of control plane and data plane
* Microservices-based architecture
* Eventually consistent system

---

## 🛠️ Tech Stack

* **Language:** Go (Golang)
* **Communication:** HTTP (REST + Streaming)
* **Container Runtime:** Docker
* **Concurrency:** Goroutines & Channels
* **Architecture:** Microservices

---

## 📦 Project Structure

```
cmd/
  api-server/
  scheduler/
  controller/
  node-agent/
  service-proxy/

pkg/
  models/
  client/
```

---

## 🚀 How It Works

1. User creates a Deployment via API Server
2. Controller creates required Pods
3. Scheduler assigns Pods to Nodes
4. Node Agent runs containers using Docker
5. Service Proxy routes traffic to running Pods
6. Watch API streams updates to all components

---

## 🧪 Example Flow

```
Create Deployment (replicas=3)
        ↓
Controller creates 3 Pods
        ↓
Scheduler assigns nodes
        ↓
Node Agent runs containers
        ↓
Service Proxy load balances traffic
```

---

## ⚠️ Limitations

* No persistent storage (in-memory state)
* No strong consistency guarantees
* No event replay (Level-1 watch system)
* No authentication/authorization
* Basic networking (localhost-based)

---

## 🔮 Future Improvements

### 🧠 Advanced Scheduling

* Pod spreading / anti-affinity
* Priority-based scheduling
* Preemption

### ⚡ System Reliability

* Leader election (multi-scheduler support)
* Distributed locking
* Fault-tolerant controllers

### 🗄️ Storage Layer

* etcd-like persistent key-value store
* Resource versioning & event replay

### 🌐 Networking

* Service DNS (e.g., `nginx-svc.local`)
* Overlay networking
* Ingress support

### 🔄 Event System

* WebSocket-based streaming
* Backpressure handling
* Event versioning

---

## 💡 Learnings

This project helped in understanding:

* Distributed system design
* Event-driven architecture
* Kubernetes internals (Scheduler, Controller, Watch API)
* Concurrency patterns in Go
* Microservices communication

---

## 🧑‍💻 Author

**Vaibhav Pathak**

---

## ⭐ If you like this project

Give it a star ⭐ and feel free to contribute!
