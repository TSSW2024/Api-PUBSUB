package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

// Estructura para almacenar los mensajes en memoria
type MessageStore struct {
	messages []string
	mu       sync.RWMutex
}

func (store *MessageStore) AddMessage(message string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.messages = append(store.messages, message)
}

func (store *MessageStore) GetMessages() []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
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

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello World!")
	})

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

	r.GET("/messages", func(c *gin.Context) {
		messages := store.GetMessages()
		c.JSON(http.StatusOK, gin.H{"messages": messages})
	})

	r.GET("/notify-check-coins", func(c *gin.Context) {
		message := "Revisa tus monedas de seguimiento si han cambiado"
		err := publishMessage(c.Request.Context(), pubsubClient, "my-topic", message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"status": "success", "message": message})
		}
	})

	r.GET("/notify-open-box", func(c *gin.Context) {
		message := "Ten tu oportunidad de jugar y abrir una caja"
		err := publishMessage(c.Request.Context(), pubsubClient, "my-topic", message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"status": "success", "message": message})
		}
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

// Suscribirse a mensajes de Pub/Sub y almacenar todos los mensajes
func subscribeMessages(ctx context.Context, client *pubsub.Client, subscriptionName string, store *MessageStore) {
	sub := client.Subscription(subscriptionName)
	err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		log.Printf("Got message: %q\n", string(msg.Data))
		store.AddMessage(string(msg.Data))
		msg.Ack()
	})
	if err != nil {
		log.Fatalf("Failed to receive messages: %v", err)
	}
}

// Función para enviar notificaciones periódicas
func startPeriodicNotifications(pubsubClient *pubsub.Client, topicName string) {
	tickerCheckCoins := time.NewTicker(8 * time.Hour) // Notificaciones cada 8 horas
	tickerOpenBox := time.NewTicker(10 * time.Hour)   // Notificaciones cada 10 horas
	defer tickerCheckCoins.Stop()
	defer tickerOpenBox.Stop()

	for {
		select {
		case <-tickerCheckCoins.C:
			sendNotification(pubsubClient, topicName, "Revisa tus monedas de seguimiento si han cambiado")
		case <-tickerOpenBox.C:
			sendNotification(pubsubClient, topicName, "Ten tu oportunidad de jugar y abrir una caja")
		}
	}
}

func sendNotification(pubsubClient *pubsub.Client, topicName string, message string) {
	// Enviar notificación
	err := publishMessage(context.Background(), pubsubClient, topicName, message)
	if err != nil {
		log.Printf("Failed to send notification: %v", err)
	} else {
		log.Printf("Sent notification: %s", message)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	credentialsFile := "service_account.json" // Usar el archivo JSON de la cuenta de servicio generado

	client, err := pubsub.NewClient(ctx, projectID, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	store := &MessageStore{}

	r := setupRouter(client, store)
	go func() {
		if err := r.Run(":8090"); err != nil {
			log.Fatalf("Failed to run server: %v", err)
		}
	}()

	go startPeriodicNotifications(client, "my-topic")

	subscribeMessages(ctx, client, "my-sub", store)
}
