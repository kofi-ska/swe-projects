.PHONY: test race bench run compose-up compose-down image k3s-import k3s-deploy k3d-create k3d-import

K3D_CLUSTER ?= llm-gateway
test:
	go test ./...
race:
	go test -race ./...
bench:
	go test -bench=. -benchmem ./...
run:
	FAILURE_RATE=0 go run ./cmd/gateway
compose-up:
	docker compose up --build
compose-down:
	docker compose down
image:
	docker build -t llm-gateway:local .
k3s-import: image
	docker save llm-gateway:local | sudo k3s ctr images import -
k3s-deploy:
	kubectl apply -k .
k3d-create:
	k3d cluster create $(K3D_CLUSTER) --servers 1 --agents 1 --wait
k3d-import: image
	k3d image import llm-gateway:local -c $(K3D_CLUSTER)
