# Fast Pizza 

## Fast Pizza - microservice app, in which are used:
1) gRPC/Protobuf
2) RabbitMQ
3) Gin(API)
4) Docker Containers

## Interaction Schema
Client -> WEB(API) -> gRPC -> RabbitMQ -> worker -> vise versa

## Running in root of project
```bash
make run
```

### Routes to interact with app
```bash
http://localhost:80/fib?n=20
http://localhost:80/order (need some json data)
http://localhost:80/order/123
http://localhost:15672 (rabbitmq management)
```
