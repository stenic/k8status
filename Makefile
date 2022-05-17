NS = default
ARGS = 

HELM_DOCS = $(shell pwd)/bin/helm-docs
helm-docs: ## Download helm-docs locally if necessary.
	$(call go-get-tool,$(HELM_DOCS),github.com/norwoodj/helm-docs/cmd/helm-docs@v1.6.0)

run:
	docker build -t k8status .
	docker run -ti -p 8080:8080 -v ~/.kube:/home/nonroot/.kube k8status --namespace $(NS) $(ARGS)

.PHONY: server
server:
	cd server \
		&& go run main.go --namespace $(NS)

.PHONY: ui
ui:
	cd ui \
		&& npm start

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef