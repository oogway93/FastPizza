package main

import (
	"context"
	"log"
	"net"

	pb "github.com/oogway93/FastPizza/proto"
	"github.com/oogway93/FastPizza/utils"
	"google.golang.org/grpc"
)

type FibServer struct {
	pb.UnimplementedFibonacciServer
}

func fibonacci(n int64) int64 {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func (s *FibServer) Fib(ctx context.Context, req *pb.FibReq) (*pb.FibRes, error) {
	res := fibonacci(req.N)
	return &pb.FibRes{N: res}, nil
}

func main() {
	// Запускаем gRPC сервер
	lis, err := net.Listen("tcp", ":8081")
	utils.FailOnError(err)
	s := grpc.NewServer()
	pb.RegisterFibonacciServer(s, &FibServer{})
	log.Println("gRPC server started :8081")
	s.Serve(lis)
}
