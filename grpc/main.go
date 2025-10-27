package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strconv"
	"time"

	pb "github.com/oogway93/FastPizza/proto"
	"github.com/oogway93/FastPizza/utils"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	pb.UnimplementedFibonacciServer
	pb.UnimplementedOrderServiceServer
	rabbitConn *amqp.Connection
}

// Fibonacci service implementation
func (s *GRPCServer) Fib(ctx context.Context, req *pb.FibReq) (*pb.FibRes, error) {
	ch, err := s.rabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	replyQueue, err := ch.QueueDeclare(
		"",    // auto-generated name
		false, // durable
		true,  // autoDelete
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	taskID := "fib_" + time.Now().Format("20060102150405.000000")

	taskMsg := map[string]interface{}{
		"task_id":  taskID,
		"type":     "fibonacci",
		"n":        req.N,
		"reply_to": replyQueue.Name,
	}

	taskBody, err := json.Marshal(taskMsg)
	if err != nil {
		return nil, err
	}

	err = ch.Publish(
		"",          // exchange
		"tasks",     // routing key (общая очередь)
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        taskBody,
		})
	if err != nil {
		return nil, err
	}

	log.Printf("Fibonacci task %s submitted for n=%d", taskID, req.N)

	// Wait for result
	msgs, err := ch.Consume(
		replyQueue.Name, // queue
		"",              // consumer
		true,            // auto-ack
		true,            // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	if err != nil {
		return nil, err
	}

	select {
	case d := <-msgs:
		var resultMsg struct {
			TaskID string `json:"task_id"`
			Result int64  `json:"result"`
			Error  string `json:"error,omitempty"`
		}
		if err := json.Unmarshal(d.Body, &resultMsg); err != nil {
			return nil, err
		}

		if resultMsg.TaskID != taskID {
			log.Printf("Task ID mismatch: expected %s, got %s", taskID, resultMsg.TaskID)
			return nil, err
		}

		if resultMsg.Error != "" {
			log.Printf("Error from worker: %s", resultMsg.Error)
			return nil, err
		}

		log.Printf("Fibonacci task %s completed with result: %d", taskID, resultMsg.Result)
		return &pb.FibRes{N: resultMsg.Result}, nil

	case <-time.After(30 * time.Second):
		log.Printf("Timeout waiting for task %s", taskID)
		return nil, err

	case <-ctx.Done():
		log.Printf("Context cancelled for task %s", taskID)
		return nil, ctx.Err()
	}
}

// OrderService implementation
func (s *GRPCServer) MakeOrder(ctx context.Context, req *pb.OrderInfo) (*pb.OrderID, error) {
	ch, err := s.rabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	replyQueue, err := ch.QueueDeclare(
		"",    // auto-generated name
		false, // durable
		true,  // autoDelete
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	orderID := time.Now().Unix()
	taskID := "order_" + strconv.FormatInt(orderID, 10)

	taskMsg := map[string]interface{}{
		"task_id":   taskID,
		"type":      "make_order",
		"order_id":  orderID,
		"username":  req.Cred.Username,
		"email":     req.Cred.Email,
		"pizza":     req.Menu.Pizza,
		"price":     req.Menu.Price,
		"reply_to":  replyQueue.Name,
	}

	taskBody, err := json.Marshal(taskMsg)
	if err != nil {
		return nil, err
	}

	err = ch.Publish(
		"",      // exchange
		"tasks", // routing key (общая очередь)
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        taskBody,
		})
	if err != nil {
		return nil, err
	}

	log.Printf("Order task %s submitted for user %s", taskID, req.Cred.Username)

	// Wait for order processing confirmation
	msgs, err := ch.Consume(
		replyQueue.Name, // queue
		"",              // consumer
		true,            // auto-ack
		true,            // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	if err != nil {
		return nil, err
	}

	select {
	case d := <-msgs:
		var resultMsg struct {
			TaskID  string `json:"task_id"`
			OrderID int64  `json:"order_id"`
			Status  string `json:"status"`
			Error   string `json:"error,omitempty"`
		}
		if err := json.Unmarshal(d.Body, &resultMsg); err != nil {
			return nil, err
		}

		if resultMsg.TaskID != taskID {
			log.Printf("Task ID mismatch: expected %s, got %s", taskID, resultMsg.TaskID)
			return nil, err
		}

		if resultMsg.Error != "" {
			log.Printf("Error from worker: %s", resultMsg.Error)
			return nil, err
		}

		log.Printf("Order task %s completed with status: %s", taskID, resultMsg.Status)
		return &pb.OrderID{OrderId: resultMsg.OrderID}, nil

	case <-time.After(30 * time.Second):
		log.Printf("Timeout waiting for order task %s", taskID)
		return nil, err

	case <-ctx.Done():
		log.Printf("Context cancelled for order task %s", taskID)
		return nil, ctx.Err()
	}
}

func (s *GRPCServer) GetStatus(ctx context.Context, req *pb.OrderID) (*pb.OrderInfo, error) {
	// For simplicity, we'll implement a basic in-memory storage
	// In production, you would use Redis or database
	return &pb.OrderInfo{
		// Status:  "preparing",
		Cred: &pb.Credentials{
			Username: "test_user", // This would come from storage
			Email:    "test@example.com",
		},
		Menu: &pb.OrderedMenu{
			Pizza: "Margherita", // This would come from storage
			Price: 12.99,
		},
	}, nil
}

func main() {
	// Connect to RabbitMQ
	rabbitConn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer rabbitConn.Close()

	// Start gRPC server
	lis, err := net.Listen("tcp", ":8081")
	utils.FailOnError(err, "Listening port:8081 isn't success")
	
	s := grpc.NewServer()
	server := &GRPCServer{rabbitConn: rabbitConn}
	pb.RegisterFibonacciServer(s, server)
	pb.RegisterOrderServiceServer(s, server)
	
	log.Println("gRPC server started on :8081")
	log.Println("Services: Fibonacci, OrderService")
	s.Serve(lis)
}