run:
	sudo docker compose up --build
stop:
	sudo docker compose down 
gen:
	protoc --proto_path=proto --go_out=proto --go-grpc_out=proto proto/*.proto
