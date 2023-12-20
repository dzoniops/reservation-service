build:
	go build -o bin/app

run: build
	./bin/app

test:
	go test -v ./.. -count=1

proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative user/user.proto

update:
	go clean -modcache
	go get -u github.com/dzoniops/...
update-all:
	go clean -modcache
	go get -u all 