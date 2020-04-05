all:	
	go build -o ./ws ./http.go ./client.go ./handle.go ./main.go
	./ws -ledisAddr=:6380

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../ws/build/bin/ws-linux-64 ./client.go ./handle.go ./http.go ./main.go

