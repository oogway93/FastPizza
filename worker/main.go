package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/oogway93/FastPizza/utils"
	amqp "github.com/rabbitmq/amqp091-go"
)

var orderStorage = make(map[int64]map[string]interface{})

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

// Process Fibonacci task
func processFibTask(taskData map[string]interface{}, ch *amqp.Channel, d amqp.Delivery) {
	taskID := taskData["task_id"].(string)
	n := int64(taskData["n"].(float64))
	replyTo := taskData["reply_to"].(string)

	log.Printf("Processing Fibonacci(%d) for task %s", n, taskID)

	// Process with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	resultChan := make(chan int64)

	go func() {
		result := fibonacci(n)
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		// Send result back
		resultMsg := map[string]interface{}{
			"task_id": taskID,
			"result":  result,
		}
		resultBody, err := json.Marshal(resultMsg)
		if err != nil {
			log.Printf("Error marshaling result: %v", err)
			d.Nack(false, true)
			return
		}

		err = ch.Publish("", replyTo, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        resultBody,
		})
		if err != nil {
			log.Printf("Error publishing result: %v", err)
			d.Nack(false, true)
			return
		}

		d.Ack(false)
		log.Printf("Completed Fibonacci(%d) = %d for task %s", n, result, taskID)

	case <-ctx.Done():
		log.Printf("Task %s timeout for Fibonacci(%d), rejecting message", taskID, n)
		d.Nack(false, false)
	}
}

// Process Order task
func processOrderTask(taskData map[string]interface{}, ch *amqp.Channel, d amqp.Delivery) {
	taskID := taskData["task_id"].(string)
	orderID := int64(taskData["order_id"].(float64))
	username := taskData["username"].(string)
	email := taskData["email"].(string)
	pizza := taskData["pizza"].(string)
	price := taskData["price"].(float64)
	replyTo := taskData["reply_to"].(string)

	log.Printf("Processing order %d for user %s", orderID, username)

	// Store order 
	orderStorage[orderID] = map[string]interface{}{
		"username": username,
		"email":    email,
		"pizza":    pizza,
		"price":    price,
		"status":   "confirmed",
	}

	// Send confirmation back
	resultMsg := map[string]interface{}{
		"task_id":  taskID,
		"order_id": orderID,
		"status":   "confirmed",
	}
	resultBody, err := json.Marshal(resultMsg)
	if err != nil {
		log.Printf("Error marshaling order result: %v", err)
		d.Nack(false, true)
		return
	}

	err = ch.Publish("", replyTo, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        resultBody,
	})
	if err != nil {
		log.Printf("Error publishing order result: %v", err)
		d.Nack(false, true)
		return
	}

	d.Ack(false)
	log.Printf("Order %d confirmed for user %s", orderID, username)
}

func main() {
	log.Println("Connecting to RabbitMQ...")
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	utils.FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()
	log.Println("Successfully connected to RabbitMQ")

	ch, err := conn.Channel()
	utils.FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	// Declare tasks queue
	q, err := ch.QueueDeclare(
		"tasks", // queue name
		true,    // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	utils.FailOnError(err, "Failed to declare queue")

	// Set QoS to handle multiple tasks
	err = ch.Qos(
		3,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	utils.FailOnError(err, "Failed to set QoS")

	// Start consuming
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	utils.FailOnError(err, "Failed to register consumer")

	log.Printf("Worker started. Waiting for messages on queue '%s'...", q.Name)

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			var taskData map[string]interface{}
			if err := json.Unmarshal(d.Body, &taskData); err != nil {
				log.Printf("Error unmarshaling task: %v", err)
				d.Nack(false, false)
				continue
			}

			taskType, exists := taskData["type"]
			if !exists {
				log.Printf("Task type not specified")
				d.Nack(false, false)
				continue
			}

			// Route to appropriate handler
			switch taskType {
			case "fibonacci":
				go processFibTask(taskData, ch, d)
			case "make_order":
				go processOrderTask(taskData, ch, d)
			default:
				log.Printf("Unknown task type: %s", taskType)
				d.Nack(false, false)
			}
		}
	}()

	<-forever
}
