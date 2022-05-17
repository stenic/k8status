# K8Status

[![Last release](https://github.com/stenic/k8status/actions/workflows/release.yaml/badge.svg)](https://github.com/stenic/k8status/actions/workflows/release.yaml)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/k8status)](https://artifacthub.io/packages/search?repo=k8status)


K8Status collects status information about services in your namespace and exposes them using a json api.

## Installation

```sh
helm repo add k8status https://stenic.github.io/k8status/
helm install k8status --namespace mynamespace k8status/k8status
```

## Build & run

```
docker build -t k8status .
docker run -ti -p 8080:8080 -v ~/.kube:/home/nonroot/.kube k8status --namespace default
```