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
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Client manages RabbitMQ connections and queue operations.
type Client struct {
	uri         string
	conn        *amqp.Connection
	mu          sync.RWMutex
	notifyClose chan *amqp.Error
}

// NewClient creates a new RabbitMQ client.
func NewClient(uri string) *Client {
	return &Client{
		uri: uri,
	}
}

// Connect establishes a connection to RabbitMQ.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	logger := log.FromContext(ctx)

	if c.conn != nil && !c.conn.IsClosed() {
		return nil
	}

	conn, err := amqp.Dial(c.uri)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	c.conn = conn
	c.notifyClose = conn.NotifyClose(make(chan *amqp.Error, 1))

	logger.Info("Connected to RabbitMQ")
	return nil
}

// Close closes the RabbitMQ connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && !c.conn.IsClosed() {
		return c.conn.Close()
	}
	return nil
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.conn.IsClosed()
}

// Channel returns a new channel from the connection.
func (c *Client) Channel(ctx context.Context) (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil || c.conn.IsClosed() {
		return nil, fmt.Errorf("not connected to RabbitMQ")
	}

	return c.conn.Channel()
}

// SourceQueueConfig contains configuration for declaring a source queue.
type SourceQueueConfig struct {
	QueueName  string
	Exchange   string
	RoutingKey string
}

// DeclareSourceQueue declares a source queue and binds it to an exchange.
func (c *Client) DeclareSourceQueue(ctx context.Context, cfg SourceQueueConfig) error {
	logger := log.FromContext(ctx)

	ch, err := c.Channel(ctx)
	if err != nil {
		return err
	}
	defer func(ch *amqp.Channel) {
		err := ch.Close()
		if err != nil {
			fmt.Printf("Failed to close channel: %v\n", err)
		}
	}(ch)

	// Declare the queue as durable
	_, err = ch.QueueDeclare(
		cfg.QueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare source queue %s: %w", cfg.QueueName, err)
	}

	// Bind the queue to the exchange
	err = ch.QueueBind(
		cfg.QueueName,
		cfg.RoutingKey,
		cfg.Exchange,
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", cfg.QueueName, cfg.Exchange, err)
	}

	logger.Info("Declared and bound source queue",
		"queue", cfg.QueueName,
		"exchange", cfg.Exchange,
		"routingKey", cfg.RoutingKey)

	return nil
}

// TargetQueueConfig contains configuration for declaring a target ring-buffer queue.
type TargetQueueConfig struct {
	QueueName  string
	MaxLength  int32
	TTLSeconds int32
}

// DeclareTargetQueue declares a target queue with ring-buffer semantics.
func (c *Client) DeclareTargetQueue(ctx context.Context, cfg TargetQueueConfig) error {
	logger := log.FromContext(ctx)

	ch, err := c.Channel(ctx)
	if err != nil {
		return err
	}
	defer func(ch *amqp.Channel) {
		err := ch.Close()
		if err != nil {
			fmt.Printf("Failed to close channel: %v\n", err)
		}
	}(ch)

	args := amqp.Table{
		"x-max-length":  cfg.MaxLength,
		"x-overflow":    "drop-head",
		"x-message-ttl": cfg.TTLSeconds * 1000, // Convert to milliseconds
	}

	// Try to declare the queue
	_, err = ch.QueueDeclare(
		cfg.QueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		args,
	)
	if err != nil {
		// If queue exists with different arguments, delete and recreate
		var amqpErr *amqp.Error
		if errors.As(err, &amqpErr) && amqpErr.Code == amqp.PreconditionFailed {
			logger.Info("Target queue exists with different arguments, recreating",
				"queue", cfg.QueueName)

			// Need a new channel since the previous one is closed after the error
			ch2, err2 := c.Channel(ctx)
			if err2 != nil {
				return fmt.Errorf("failed to get channel for queue deletion: %w", err2)
			}
			defer func(ch2 *amqp.Channel) {
				err := ch2.Close()
				if err != nil {
					fmt.Printf("Failed to close channel: %v\n", err)
				}
			}(ch2)

			// Delete the existing queue
			_, err2 = ch2.QueueDelete(cfg.QueueName, false, false, false)
			if err2 != nil {
				return fmt.Errorf("failed to delete existing queue %s: %w", cfg.QueueName, err2)
			}

			// Recreate with new arguments
			_, err2 = ch2.QueueDeclare(
				cfg.QueueName,
				true,
				false,
				false,
				false,
				args,
			)
			if err2 != nil {
				return fmt.Errorf("failed to recreate target queue %s: %w", cfg.QueueName, err2)
			}
		}
	}

	logger.Info("Declared target queue with ring-buffer semantics",
		"queue", cfg.QueueName,
		"maxLength", cfg.MaxLength,
		"ttlSeconds", cfg.TTLSeconds)

	return nil
}

// DeleteQueue deletes a queue from RabbitMQ.
func (c *Client) DeleteQueue(ctx context.Context, queueName string) error {
	logger := log.FromContext(ctx)

	ch, err := c.Channel(ctx)
	if err != nil {
		return err
	}
	defer func(ch *amqp.Channel) {
		err := ch.Close()
		if err != nil {
			fmt.Printf("Failed to close channel: %v\n", err)
		}
	}(ch)

	_, err = ch.QueueDelete(queueName, false, false, false)
	if err != nil {
		// Ignore "not found" errors during cleanup
		var amqpErr *amqp.Error
		if errors.As(err, &amqpErr) && amqpErr.Code == amqp.NotFound {
			logger.V(1).Info("Queue already deleted or doesn't exist", "queue", queueName)
			return nil
		}
		return fmt.Errorf("failed to delete queue %s: %w", queueName, err)
	}

	logger.Info("Deleted queue", "queue", queueName)
	return nil
}

// ConnectWithRetry connects to RabbitMQ with exponential backoff.
func (c *Client) ConnectWithRetry(ctx context.Context, maxRetries int) error {
	logger := log.FromContext(ctx)

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for i := range maxRetries {
		err := c.Connect(ctx)
		if err == nil {
			return nil
		}

		logger.Error(err, "Failed to connect to RabbitMQ, retrying",
			"attempt", i+1,
			"maxRetries", maxRetries,
			"backoff", backoff)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return fmt.Errorf("failed to connect to RabbitMQ after %d retries", maxRetries)
}
