# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying the PUBSUB server.

## Quick Start

### Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured  
- Docker image built and available

### Basic Deployment

1. Build and load the Docker image:
   ```bash
   docker build -t pubsub:latest -f server/Dockerfile .
   # For Minikube:
   minikube image load pubsub:latest
   # For Kind:
   kind load docker-image pubsub:latest
   ```

2. Deploy to Kubernetes:
   ```bash
   kubectl apply -f k8s/deployment.yaml
   ```

3. Verify deployment:
   ```bash
   kubectl get pods -n pubsub
   kubectl get svc -n pubsub
   ```

4. Check health:
   ```bash
   kubectl port-forward -n pubsub svc/pubsub-server 8080:8080
   curl http://localhost:8080/health
   ```

## Configuration

The deployment includes:
- **Namespace**: `pubsub`
- **Deployment**: 3 replicas with health checks
- **Service**: ClusterIP exposing ports 9999 (pubsub) and 8080 (health)
- **HPA**: Auto-scaling based on CPU/memory

## Security

The deployment follows security best practices:
- Non-root user (UID 65532)
- Read-only root filesystem
- All capabilities dropped
- seccomp profile enabled

## Monitoring

Health check endpoints:
- `/health` - liveness probe
- `/ready` - readiness probe

View logs:
```bash
kubectl logs -n pubsub -l app=pubsub -f
```

## Scaling

Manual:
```bash
kubectl scale deployment/pubsub-server --replicas=5 -n pubsub
```

Auto-scaling is configured via HPA.

## Cleanup

```bash
kubectl delete namespace pubsub
```
