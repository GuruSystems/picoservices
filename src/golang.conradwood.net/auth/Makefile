PROTOCINC = -I.
PROTOCINC += -I${GOPATH}/src/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/third_party/protobuf/src/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/third_party/
PROTOCINC += -I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis

PKG=auth

server:
	echo "no server for auth lib"
client:
	echo "no client for auth lib"

all:
	echo nothing todo 


.PHONY: proto
proto:
	@echo compiling Go proto stubs
	@protoc ${PROTOCINC} --go_out=plugins=grpc:. proto/${PKG}.proto
	# for json gateway to compile you need the plugins:
	# 1. protoc-gen-swagger
	# 2. protoc-gen-grpc-gateway
	# they are in package github.com/grpc-ecosystem/grpc-gateway
	@protoc ${PROTOCINC} --java_out=. proto/${PKG}.proto

