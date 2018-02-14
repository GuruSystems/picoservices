PROTOCINC = -I.
PROTOCINC += -I${GOPATH}/src/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/third_party/protobuf/src/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/
PROTOCINC += -I${GOPATH}/src/github.com/googleapis/googleapis/third_party/
PROTOCINC += -I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis

PKG=auth

server:
	go install ${PKG}-server.go auth-postgres.go auth-file.go auth-any.go auth-nil.go auth-ldap.go auth-psql-ldap.go
client:
	go install ${PKG}-client.go 
	go install login.go
proto:
	@echo no proto for server
all:
	make proto
	make client server
