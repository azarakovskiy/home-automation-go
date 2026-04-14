package entities

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type runtimeMessageHandler func(context.Context, string, []byte)

type runtimeTransport interface {
	SetOnConnect(func(context.Context) error)
	Connect(context.Context) error
	Publish(context.Context, string, bool, []byte) error
	Subscribe(context.Context, string, runtimeMessageHandler) error
	Close() error
}

type pahoRuntimeTransport struct {
	client mqtt.Client

	mu            sync.RWMutex
	onConnect     func(context.Context) error
	subscriptions map[string]runtimeMessageHandler
}

func newPahoRuntimeTransport(cfg RuntimeConfig, availabilityTopic string) (*pahoRuntimeTransport, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(runtimeClientID(cfg)).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5*time.Second).
		SetWriteTimeout(10*time.Second).
		SetOrderMatters(false).
		SetWill(availabilityTopic, runtimeAvailabilityOffline, 1, true)

	transport := &pahoRuntimeTransport{
		subscriptions: make(map[string]runtimeMessageHandler),
	}

	opts.OnConnect = func(client mqtt.Client) {
		transport.restoreSubscriptions(context.Background())

		transport.mu.RLock()
		onConnect := transport.onConnect
		transport.mu.RUnlock()
		if onConnect != nil {
			if err := onConnect(context.Background()); err != nil {
				log.Printf("WARNING: runtime MQTT on-connect callback failed: %v", err)
			}
		}
	}

	transport.client = mqtt.NewClient(opts)
	return transport, nil
}

func (t *pahoRuntimeTransport) SetOnConnect(fn func(context.Context) error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onConnect = fn
}

func (t *pahoRuntimeTransport) Connect(_ context.Context) error {
	token := t.client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return err
	}
	return nil
}

func (t *pahoRuntimeTransport) Publish(_ context.Context, topic string, retained bool, payload []byte) error {
	token := t.client.Publish(topic, 1, retained, payload)
	token.Wait()
	if err := token.Error(); err != nil {
		return err
	}
	return nil
}

func (t *pahoRuntimeTransport) Subscribe(_ context.Context, topic string, handler runtimeMessageHandler) error {
	t.mu.Lock()
	t.subscriptions[topic] = handler
	t.mu.Unlock()

	if err := t.subscribe(context.Background(), topic, handler); err != nil {
		// If the connection was lost the subscription is already registered in t.subscriptions.
		// OnConnect will restore it once the client reconnects, so this is not fatal.
		if isTransientConnectionError(err) {
			log.Printf("DEBUG: subscribe to %s deferred until reconnect: %v", topic, err)
			return nil
		}
		return err
	}
	return nil
}

func (t *pahoRuntimeTransport) Close() error {
	if t.client.IsConnected() {
		t.client.Disconnect(250)
	}
	return nil
}

func (t *pahoRuntimeTransport) restoreSubscriptions(ctx context.Context) {
	t.mu.RLock()
	subscriptions := make(map[string]runtimeMessageHandler, len(t.subscriptions))
	for topic, handler := range t.subscriptions {
		subscriptions[topic] = handler
	}
	t.mu.RUnlock()

	for topic, handler := range subscriptions {
		if err := t.subscribe(ctx, topic, handler); err != nil {
			// Log and continue: a failed restore for one topic must not block
			// the rest or prevent the onConnect callback from running.
			log.Printf("WARNING: failed to restore subscription for %s: %v", topic, err)
		}
	}
}

func (t *pahoRuntimeTransport) subscribe(_ context.Context, topic string, handler runtimeMessageHandler) error {
	token := t.client.Subscribe(topic, 1, func(_ mqtt.Client, message mqtt.Message) {
		handler(context.Background(), message.Topic(), append([]byte(nil), message.Payload()...))
	})
	token.Wait()
	if err := token.Error(); err != nil {
		if strings.Contains(err.Error(), "already subscribed") {
			return nil
		}
		return fmt.Errorf("subscribe %s: %w", topic, err)
	}
	return nil
}

func isTransientConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection lost") ||
		strings.Contains(msg, "not connected") ||
		strings.Contains(msg, "not currently connected") ||
		strings.Contains(msg, "EOF")
}

func runtimeClientID(cfg RuntimeConfig) string {
	if strings.TrimSpace(cfg.ClientID) != "" {
		return cfg.ClientID
	}
	return cfg.AppPrefix
}
