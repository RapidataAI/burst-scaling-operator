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
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PublishBurst publishes count messages to the target queue via the default exchange.
// Messages are minimal with a timestamp payload.
func PublishBurst(ctx context.Context, ch *amqp.Channel, targetQueueName string, count int32) error {
	logger := log.FromContext(ctx)

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	body := []byte(timestamp)

	for i := range count {
		err := ch.PublishWithContext(
			ctx,
			"",              // exchange (default exchange)
			targetQueueName, // routing key (queue name for default exchange)
			false,           // mandatory
			false,           // immediate
			amqp.Publishing{
				ContentType:  "text/plain",
				Body:         body,
				DeliveryMode: amqp.Transient, // Non-persistent since TTL will expire anyway
				Timestamp:    time.Now(),
			},
		)
		if err != nil {
			return fmt.Errorf("failed to publish message %d/%d: %w", i+1, count, err)
		}
	}

	logger.V(1).Info("Published burst messages",
		"count", count,
		"queue", targetQueueName)

	return nil
}

// PublishBurstWithConnection creates a connection and publishes burst messages.
// Use this for one-off publishing when you don't have an existing channel.
func PublishBurstWithConnection(ctx context.Context, amqpURI, targetQueueName string, count int32) error {
	conn, err := amqp.Dial(amqpURI)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func(conn *amqp.Connection) {
		err := conn.Close()
		if err != nil {
			fmt.Printf("Failed to close connection: %v\n", err)
		}
	}(conn)

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer func(ch *amqp.Channel) {
		err := ch.Close()
		if err != nil {
			fmt.Printf("Failed to close channel: %v\n", err)
		}
	}(ch)

	return PublishBurst(ctx, ch, targetQueueName, count)
}
