package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"time"

	pb "github.com/oogway93/FastPizza/proto"
	"github.com/oogway93/FastPizza/utils"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
)

type FibServer struct {
	pb.UnimplementedFibonacciServer
	rabbitConn *amqp.Connection
}

type TaskMessage struct {
	TaskID    string `json:"task_id"`
	N         int64  `json:"n"`
	ReplyTo   string `json:"reply_to"`
	Timestamp int64  `json:"timestamp"`
}

type ResultMessage struct {
	TaskID string `json:"task_id"`
	Result int64  `json:"result"`
	Error  string `json:"error,omitempty"`
}

func (s *FibServer) Fib(ctx context.Context, req *pb.FibReq) (*pb.FibRes, error) {
	ch, err := s.rabbitConn.Channel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	// Создаем временную очередь для ответа
	replyQueue, err := ch.QueueDeclare(
		"",    // пустое имя - сервер сгенерирует уникальное
		false, // durable
		true,  // autoDelete
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, err
	}

	// Генерируем ID задачи
	taskID := "fib_" + time.Now().Format("20060102150405.000000")

	// Подготавливаем сообщение
	taskMsg := TaskMessage{
		TaskID:    taskID,
		N:         req.N,
		ReplyTo:   replyQueue.Name,
		Timestamp: time.Now().UnixNano(),
	}

	// Сериализуем в JSON
	taskBody, err := json.Marshal(taskMsg)
	if err != nil {
		return nil, err
	}

	// Публикуем задачу в RabbitMQ
	err = ch.Publish(
		"",          // exchange
		"fib_tasks", // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        taskBody,
		})
	if err != nil {
		return nil, err
	}

	log.Printf("Task %s submitted for Fibonacci(%d), waiting for result...", taskID, req.N)

	// Ждем результат из временной очереди
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

	// Ждем результат с таймаутом
	select {
	case d := <-msgs:
		var resultMsg ResultMessage
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

		log.Printf("Task %s completed with result: %d", taskID, resultMsg.Result)
		return &pb.FibRes{N: resultMsg.Result}, nil

	case <-time.After(30 * time.Second):
		log.Printf("Timeout waiting for task %s", taskID)
		return nil, err

	case <-ctx.Done():
		log.Printf("Context cancelled for task %s", taskID)
		return nil, ctx.Err()
	}
}

func main() {
	rabbitConn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	utils.FailOnError(err)
	defer rabbitConn.Close()
	// Запускаем gRPC сервер
	lis, err := net.Listen("tcp", ":8081")
	utils.FailOnError(err)
	s := grpc.NewServer()
	pb.RegisterFibonacciServer(s, &FibServer{rabbitConn: rabbitConn})
	log.Println("gRPC server started :8081")
	s.Serve(lis)
}
