package main

import (
	"context"
	"strconv"
	// "time"

	"github.com/gin-gonic/gin"
	pb "github.com/oogway93/FastPizza/proto"
	"github.com/oogway93/FastPizza/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Handler struct {
	client pb.FibonacciClient
}

func NewHandler(fibClient pb.FibonacciClient) *Handler {
	return &Handler{client: fibClient}
}

func (h *Handler) fibonacciHandler(c *gin.Context) {
	n := c.Query("n")
	N, err := strconv.Atoi(n)
	utils.FailOnError(err)
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	ctx := context.Background()
	// defer cancel()
	response, err := h.client.Fib(ctx, &pb.FibReq{N: int64(N)})
	utils.FailOnError(err)

	c.JSON(200, map[string]int{"result": int(response.GetN())})

}

func main() {
	conn, _ := grpc.NewClient("grpc-server:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()
	client := pb.NewFibonacciClient(conn)
	handler := NewHandler(client)
	r := gin.Default()
	r.GET("/fib", handler.fibonacciHandler)
	r.Run()
}
