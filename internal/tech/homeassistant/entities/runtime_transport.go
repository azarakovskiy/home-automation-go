package entities

import (
	"context"
	"fmt"
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
		if err := transport.restoreSubscriptions(context.Background()); err != nil {
			return
		}

		transport.mu.RLock()
		onConnect := transport.onConnect
		transport.mu.RUnlock()
		if onConnect != nil {
			_ = onConnect(context.Background())
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

	return t.subscribe(context.Background(), topic, handler)
}

func (t *pahoRuntimeTransport) Close() error {
	if t.client.IsConnected() {
		t.client.Disconnect(250)
	}
	return nil
}

func (t *pahoRuntimeTransport) restoreSubscriptions(ctx context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for topic, handler := range t.subscriptions {
		if err := t.subscribe(ctx, topic, handler); err != nil {
			return err
		}
	}
	return nil
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

func runtimeClientID(cfg RuntimeConfig) string {
	if strings.TrimSpace(cfg.ClientID) != "" {
		return cfg.ClientID
	}
	return fmt.Sprintf("%s-runtime-entities", cfg.AppPrefix)
}
