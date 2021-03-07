all: init protoc build

gen: protoc build

init:
	go mod init recoverKV

protoc:
	# Generate the protobuf files using the ../proto directory
	protoc -I=../proto --go_out=. --go-grpc_out=. recoverKV.proto

build: server lb

server:
	go build -o recoverKV server.go persist.go

lb:
	go build -o recoverLB load_balancer.go

build_race:
	go build -race -o recoverKV server.go persist.go
	go build -race -o recoverLB load_balancer.go

clean:
	rm recoverKV recoverLB *.log
