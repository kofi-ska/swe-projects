.PHONY: test race bench run compose-up compose-down image k3s-import k3s-deploy
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
	kubectl apply -k k8s
