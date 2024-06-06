package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"cloud.google.com/go/pubsub"
	"github.com/gin-gonic/gin"
)

// Estructura para almacenar los mensajes en memoria
type MessageStore struct {
	messages []string
	mu       sync.Mutex
}

func (store *MessageStore) AddMessage(message string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.messages = append(store.messages, message)
}

func (store *MessageStore) GetMessages() []string {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]string{}, store.messages...)
}

// Estructura para el cuerpo JSON de la solicitud
type MessageRequest struct {
	Topic   string `json:"my-topic"`
	Message string `json:"message"`
}

// Configuración del servidor HTTP
func setupRouter(pubsubClient *pubsub.Client, store *MessageStore) *gin.Engine {
	r := gin.Default()

	// Endpoint para verificar que el servidor está corriendo
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello World!")
	})

	// Endpoint para publicar un mensaje
	r.POST("/publish", func(c *gin.Context) {
		var messageReq MessageRequest
		if err := c.BindJSON(&messageReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err := publishMessage(c.Request.Context(), pubsubClient, messageReq.Topic, messageReq.Message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"status": "success"})
		}
	})

	// Endpoint para obtener los mensajes
	r.GET("/messages", func(c *gin.Context) {
		messages := store.GetMessages()
		c.JSON(http.StatusOK, gin.H{"messages": messages})
	})

	return r
}

// Publicar un mensaje a Pub/Sub
func publishMessage(ctx context.Context, client *pubsub.Client, topicName string, message string) error {
	topic := client.Topic(topicName)
	result := topic.Publish(ctx, &pubsub.Message{
		Data: []byte(message),
	})

	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to publish: %v", err)
	}
	fmt.Printf("Published a message; msg ID: %v\n", id)
	return nil
}

// Suscribirse a mensajes de Pub/Sub
func subscribeMessages(ctx context.Context, client *pubsub.Client, subscriptionName string, store *MessageStore) {
	sub := client.Subscription(subscriptionName)
	err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		fmt.Printf("Got message: %q\n", string(msg.Data))
		store.AddMessage(string(msg.Data))
		msg.Ack()
	})
	if err != nil {
		log.Fatalf("Failed to receive messages: %v", err)
	}
}

func main() {
	// Crear el cliente Pub/Sub
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, "tss-1s2024")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Crear el almacenamiento de mensajes
	store := &MessageStore{}

	// Iniciar el servidor HTTP
	r := setupRouter(client, store)
	go func() {
		if err := r.Run(":8090"); err != nil {
			log.Fatalf("Failed to run server: %v", err)
		}
	}()

	// Suscribirse a mensajes (esto puede correr en un goroutine separado si lo deseas)
	subscribeMessages(ctx, client, "my-sub", store)
}
