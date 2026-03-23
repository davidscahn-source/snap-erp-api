run:
	go run cmd/server/main.go

build:
	go build -o bin/snap-erp-api cmd/server/main.go

tidy:
	go mod tidy

health:
	curl -s http://localhost:8080/health | python3 -m json.tool
