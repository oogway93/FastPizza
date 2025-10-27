package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/oogway93/FastPizza/utils"
	amqp "github.com/rabbitmq/amqp091-go"
)

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

func fibonacci(n int64) int64 {
	if n <= 1 {
		return n
	}

	var a, b int64 = 0, 1
	for i := int64(2); i <= n; i++ {
		a, b = b, a+b
	}
	return b
}

func processTask(taskMsg TaskMessage, ch *amqp.Channel, d amqp.Delivery) {
	log.Printf("Processing Fibonacci(%d) for task %s", taskMsg.N, taskMsg.TaskID)

	// Контекст с таймаутом 25 секунд (меньше чем gRPC таймаут)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	resultChan := make(chan int64)

	go func() {
		result := fibonacci(taskMsg.N)
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		// Успешное вычисление
		sendResult(taskMsg, result, ch, d)
	case <-ctx.Done():
		// Таймаут - отклоняем задачу без возврата в очередь
		log.Printf("Task %s timeout for Fibonacci(%d), rejecting message", taskMsg.TaskID, taskMsg.N)
		d.Nack(false, false)
	}
}

func sendResult(taskMsg TaskMessage, result int64, ch *amqp.Channel, d amqp.Delivery) {
	resultMsg := ResultMessage{
		TaskID: taskMsg.TaskID,
		Result: result,
	}

	resultBody, err := json.Marshal(resultMsg)
	if err != nil {
		log.Printf("Error marshaling result: %v", err)
		d.Nack(false, true)
		return
	}

	err = ch.Publish(
		"",
		taskMsg.ReplyTo,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        resultBody,
		})

	if err != nil {
		log.Printf("Error publishing result: %v", err)
		d.Nack(false, true)
		return
	}

	d.Ack(false)
	log.Printf("Completed Fibonacci(%d) = %d for task %s", taskMsg.N, result, taskMsg.TaskID)
}

func main() {
	time.Sleep(5 * time.Second)

	log.Println("Connecting to RabbitMQ...")
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	utils.FailOnError(err, "RabbitMQ connection unsuccess")
	defer conn.Close()
	log.Println("Successfully connected to RabbitMQ")

	ch, err := conn.Channel()
	utils.FailOnError(err, "Cannot open RabbitMQ's channel")
	defer ch.Close()

	q1, err := ch.QueueDeclare(
		"fib_tasks", // queue name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	utils.FailOnError(err, "Declaration queue isn't success")
	log.Printf("Queue '%s' declared", q1.Name)

	q2, err := ch.QueueDeclare(
		"order_tasks", // queue name
		true,        // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	utils.FailOnError(err, "Declaration queue isn't success")
	log.Printf("Queue '%s' declared", q2.Name)

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack (false - подтверждаем вручную)
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	utils.FailOnError(err, "Consuming from queue")

	log.Printf("Worker started. Waiting for messages on queue '%s'...", q.Name)

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			var taskMsg TaskMessage
			if err := json.Unmarshal(d.Body, &taskMsg); err != nil {
				log.Printf("Error unmarshaling task: %v", err)
				d.Nack(false, false) // Отклоняем без возврата
				continue
			}

			// Обрабатываем задачу в отдельной горутине
			go processTask(taskMsg, ch, d)
		}
	}()

	<-forever
}
