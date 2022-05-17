# K8Status

K8Status collects status information about services in your namespace and exposes them using a json api.


## Build & run

```
docker build -t k8status .
docker run -ti -p 8080:8080 -v ~/.kube:/home/nonroot/.kube k8status --namespace default
```