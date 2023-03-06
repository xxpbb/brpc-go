all:
	protoc --go_out=. --go_opt=paths=source_relative --go-brpc_out=. --go-brpc_opt=paths=source_relative echo.proto
	go build -o echo_client client/main.go
	go build -o echo_server server/main.go

clean:
	rm -rf *.pb.go echo_client echo_server