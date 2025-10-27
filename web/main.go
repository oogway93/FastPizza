package main

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	pb "github.com/oogway93/FastPizza/proto"
	"github.com/oogway93/FastPizza/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Handler struct {
	clientFib   pb.FibonacciClient
	clientOrder pb.OrderServiceClient
}

type OrderInfo struct {
	Pizza    string  `json:"pizza"`
	Price    float64 `json:"price"`
	Username string  `json:"username"`
	Email    string  `json:"email"`
}

func NewHandler(fibClient pb.FibonacciClient, OrderClient pb.OrderServiceClient) *Handler {
	return &Handler{clientFib: fibClient, clientOrder: OrderClient}
}

func (h *Handler) fibonacciHandler(c *gin.Context) {
	n := c.Query("n")
	N, err := strconv.Atoi(n)
	utils.FailOnError(err, "Conversion of n to type's int ")
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	ctx := context.Background()
	// defer cancel()
	response, err := h.clientFib.Fib(ctx, &pb.FibReq{N: int64(N)})
	utils.FailOnError(err, "Response in Fib rpc method")

	c.JSON(200, map[string]int{"result": int(response.GetN())})
}

func (h *Handler) makeOrderHandler(c *gin.Context) {
	orderInfo := &OrderInfo{}
	err := c.BindJSON(orderInfo)
	utils.FailOnError(err, "Binding orderInfoJSON to struct")

	isValidateEmail := utils.ValidEmail(orderInfo.Email)
	if isValidateEmail != nil {
		utils.FailOnError(isValidateEmail, "Validation Email")
	}
	ctx := context.Background()
	response, err := h.clientOrder.MakeOrder(ctx, &pb.OrderInfo{
		Cred: &pb.Credentials{
			Username: orderInfo.Username,
			Email:    orderInfo.Email,
		},
		Menu: &pb.OrderedMenu{
			Pizza: orderInfo.Pizza,
			Price: float32(orderInfo.Price),
		},
	})
	c.JSON(200, response)
}

func main() {
	conn, _ := grpc.NewClient("grpc-server:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()
	clientFib := pb.NewFibonacciClient(conn)
	clientOrder := pb.NewOrderServiceClient(conn)
	handler := NewHandler(clientFib, clientOrder)
	r := gin.Default()
	r.GET("/fib", handler.fibonacciHandler)
	r.Run()
}
