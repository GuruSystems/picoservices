PROTOCINC = -I.
PROTOCINC += -I${GOPATH}/src/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/third_party/protobuf/src/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/third_party/
PROTOCINC += -I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis

server:
	go install keyvalueserver-server.go
client:
	go install keyvalueserver-client.go
all:
	make proto
	make client server

.PHONY: proto
proto:
	@echo compiling Go proto stubs
	@protoc ${PROTOCINC} --go_out=plugins=grpc:. proto/keyvalueserver.proto
	# for json gateway to compile you need the plugins:
	# 1. protoc-gen-swagger
	# 2. protoc-gen-grpc-gateway
	# they are in package github.com/grpc-ecosystem/grpc-gateway
	@protoc ${PROTOCINC} --java_out=. proto/keyvalueserver.proto

