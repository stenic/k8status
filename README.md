# K8Status

[![Last release](https://github.com/stenic/k8status/actions/workflows/release.yaml/badge.svg)](https://github.com/stenic/k8status/actions/workflows/release.yaml)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/k8status)](https://artifacthub.io/packages/search?repo=k8status)


K8Status collects status information about services in your namespace and exposes them using a json api.

![Dashboard UI page](docs/images/dashboard-ui.png)

## Installation

```sh
helm repo add k8status https://stenic.github.io/k8status/
helm install k8status --namespace mynamespace k8status/k8status
```


## Annotations

You can add these Kubernetes annotations to specific objects to customize k8status's behaviour.

__Service__

`k8status.stenic.io/name`
(string) Overwrite the name shown in the report.

`k8status.stenic.io/exclude`
(bool) Exclude from the report.

`k8status.stenic.io/include`
(bool) Include in the report. Only used if `--mode=exclusive` is set.

`k8status.stenic.io/description`
(string) Add additional description to the service.

__Namespace__ (Only used if `--namespace` is not set)

`k8status.stenic.io/include`
(bool) Include the namespace in the report.

`k8status.stenic.io/name`
(string) Overwrite the name shown in the report.

## UI

`?mode=tv`
Render UI suited for tv's.

`?refresh=1`
Refresh the data every second.


## Build & run

```
docker build -t k8status .
docker run -ti -p 8080:8080 -v ~/.kube:/home/nonroot/.kube k8status --namespace default
```
