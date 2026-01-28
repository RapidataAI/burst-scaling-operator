/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/rapidataai/burst-scaling-operator/internal/metrics"
)

// ConsumerConfig holds the configuration for a consumer.
type ConsumerConfig struct {
	AMQPURI         string
	SourceQueueName string
	TargetQueueName string
	BurstCount      int32
}

// ConsumerWorker represents a single consumer goroutine.
type ConsumerWorker struct {
	config ConsumerConfig
	cancel context.CancelFunc
	done   chan struct{}
}

// ConsumerManager manages consumer goroutines for each BurstScaler CR.
type ConsumerManager struct {
	consumers map[types.NamespacedName]*ConsumerWorker
	mu        sync.RWMutex
}

// NewConsumerManager creates a new ConsumerManager.
func NewConsumerManager() *ConsumerManager {
	return &ConsumerManager{
		consumers: make(map[types.NamespacedName]*ConsumerWorker),
	}
}

// EnsureConsumer starts or updates a consumer for the given BurstScaler.
func (m *ConsumerManager) EnsureConsumer(ctx context.Context, key types.NamespacedName, config ConsumerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger := log.FromContext(ctx).WithValues("burstscaler", key)

	// Check if consumer exists and has same config
	if existing, ok := m.consumers[key]; ok {
		if existing.config == config {
			logger.V(1).Info("Consumer already running with same config")
			return nil
		}
		// Config changed, stop the existing consumer
		logger.Info("Consumer config changed, restarting")
		existing.cancel()
		<-existing.done
		delete(m.consumers, key)
	}

	// Start a new consumer
	consumerCtx, cancel := context.WithCancel(context.Background())
	worker := &ConsumerWorker{
		config: config,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go m.runConsumer(consumerCtx, key, worker)

	m.consumers[key] = worker
	logger.Info("Started consumer goroutine")

	return nil
}

// StopConsumer stops the consumer for the given BurstScaler.
func (m *ConsumerManager) StopConsumer(key types.NamespacedName) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if worker, ok := m.consumers[key]; ok {
		worker.cancel()
		<-worker.done
		delete(m.consumers, key)
	}
}

// IsRunning returns true if a consumer is running for the given key.
func (m *ConsumerManager) IsRunning(key types.NamespacedName) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.consumers[key]
	return ok
}

// StopAll stops all consumers.
func (m *ConsumerManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, worker := range m.consumers {
		worker.cancel()
		<-worker.done
		delete(m.consumers, key)
	}
}

// runConsumer is the main consumer loop that runs in a goroutine.
func (m *ConsumerManager) runConsumer(ctx context.Context, key types.NamespacedName, worker *ConsumerWorker) {
	defer close(worker.done)

	logger := log.FromContext(ctx).WithValues("burstscaler", key)
	config := worker.config

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			logger.Info("Consumer stopping")
			metrics.ConsumerStatus.WithLabelValues(key.Name, key.Namespace).Set(0)
			return
		default:
		}

		err := m.consumeMessages(ctx, key, config)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled, exit gracefully
				metrics.ConsumerStatus.WithLabelValues(key.Name, key.Namespace).Set(0)
				return
			}

			logger.Error(err, "Consumer error, reconnecting", "backoff", backoff)
			metrics.ConsumerStatus.WithLabelValues(key.Name, key.Namespace).Set(0)

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		} else {
			// Reset backoff on successful connection
			backoff = time.Second
		}
	}
}

// consumeMessages connects and consumes messages until an error occurs.
func (m *ConsumerManager) consumeMessages(ctx context.Context, key types.NamespacedName, config ConsumerConfig) error {
	logger := log.FromContext(ctx).WithValues("burstscaler", key)

	conn, err := amqp.Dial(config.AMQPURI)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer ch.Close()

	// Set QoS to process one message at a time
	err = ch.Qos(1, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := ch.Consume(
		config.SourceQueueName,
		"",    // consumer tag (auto-generated)
		false, // auto-ack (we'll ack manually)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	logger.Info("Consumer connected and consuming",
		"sourceQueue", config.SourceQueueName,
		"targetQueue", config.TargetQueueName)

	metrics.ConsumerStatus.WithLabelValues(key.Name, key.Namespace).Set(1)

	// Watch for connection close
	connClose := conn.NotifyClose(make(chan *amqp.Error, 1))
	chClose := ch.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-connClose:
			if err != nil {
				return fmt.Errorf("connection closed: %w", err)
			}
			return fmt.Errorf("connection closed")

		case err := <-chClose:
			if err != nil {
				return fmt.Errorf("channel closed: %w", err)
			}
			return fmt.Errorf("channel closed")

		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("message channel closed")
			}

			logger.V(1).Info("Received message, publishing burst",
				"deliveryTag", msg.DeliveryTag,
				"burstCount", config.BurstCount)

			// Publish burst messages to target queue
			err := PublishBurst(ctx, ch, config.TargetQueueName, config.BurstCount)
			if err != nil {
				logger.Error(err, "Failed to publish burst, nacking message")
				if nackErr := msg.Nack(false, true); nackErr != nil {
					logger.Error(nackErr, "Failed to nack message")
				}
				continue
			}

			// ACK the source message
			if err := msg.Ack(false); err != nil {
				logger.Error(err, "Failed to ack message")
			}

			// Update metrics
			metrics.BurstsTotal.WithLabelValues(key.Name, key.Namespace).Inc()
			metrics.SourceMessagesTotal.WithLabelValues(key.Name, key.Namespace).Inc()

			logger.Info("Burst published successfully",
				"burstCount", config.BurstCount,
				"targetQueue", config.TargetQueueName)
		}
	}
}
