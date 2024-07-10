package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
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

	var filteredMessages []string
	for _, msg := range store.messages {
		if msg == "Revisa tus monedas de seguimiento si han cambiado" {
			filteredMessages = append(filteredMessages, msg)
		}
	}
	return filteredMessages
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

	r.GET("/top-coins", func(c *gin.Context) {
		topCoins := getTopRankedCoins()
		c.JSON(http.StatusOK, gin.H{"top_coins": topCoins})
	})

	r.GET("/periodic", func(c *gin.Context) {
		sendPeriodicNotification(pubsubClient, "my-topic")
		c.JSON(http.StatusOK, gin.H{"status": "success"})
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

// Función para enviar notificaciones periódicas
func startPeriodicNotifications(pubsubClient *pubsub.Client, topicName string) {
	ticker := time.NewTicker(8 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sendPeriodicNotification(pubsubClient, topicName)
	}
}

func sendPeriodicNotification(pubsubClient *pubsub.Client, topicName string) {
	// Enviar notificación
	err := publishMessage(context.Background(), pubsubClient, topicName, "Revisa tus monedas de seguimiento si han cambiado")
	if err != nil {
		log.Printf("Failed to send periodic notification: %v", err)
	} else {
		log.Printf("Sent periodic notification: Revisa tus monedas de seguimiento si han cambiado")
	}
}

// Estructura para decodificar la respuesta de Binance
type BinanceTicker struct {
	Symbol string  `json:"symbol"`
	Volume float64 `json:"volume,string"`
}

// Función para obtener las monedas más rankeadas desde la API de Binance
func getTopRankedCoins() []string {
	resp, err := http.Get("https://api.binance.com/api/v3/ticker/24hr")
	if err != nil {
		log.Printf("Failed to fetch data from Binance: %v", err)
		return []string{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Received non-200 response from Binance: %d", resp.StatusCode)
		return []string{}
	}

	var tickers []BinanceTicker
	if err := json.NewDecoder(resp.Body).Decode(&tickers); err != nil {
		log.Printf("Failed to decode response from Binance: %v", err)
		return []string{}
	}

	sort.Slice(tickers, func(i, j int) bool {
		return tickers[i].Volume > tickers[j].Volume
	})

	topCoins := make([]string, 0, 10)
	for i := 0; i < 10 && i < len(tickers); i++ {
		topCoins = append(topCoins, tickers[i].Symbol)
	}

	return topCoins
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	ctx := context.Background()
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	credentialsFile := "service_account.json" // Nombre del archivo generado por GitHub Actions

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

	subscribeMessages(ctx, client, "projects/tss-1s2024/subscriptions/my-sub", store)
}
