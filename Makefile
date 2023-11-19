build:
	go build -o client ./cmd/client
	go build -o server ./cmd/server

deps:
	go mod download

gen:
	protoc --proto_path=proto --go_out=. --go-grpc_out=. ./proto/*.proto

clean:
	rm ./proto/*.pb.go