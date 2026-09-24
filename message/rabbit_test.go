package message

import (
	"context"
	"testing"
	"time"
)

func TestSendMessage(t *testing.T) {
	if err := InitSender("amqps://avrowstq:m6xl4MvjbRgRRhaVRc093vsJmf5yYO3z@leopard.lmq.cloudamqp.com/avrowstq"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = CloseSenders()
	}()
	if err := SendToQueue(context.Background(), "user-login-log", &Message{
		TraceID:   "2313123123213123131313asdasd1as3d13123312131d31as3d",
		MessageID: "dasdasdasdasdasd312312312dasdasdas",
		Body:      []byte(`{"orderId":1001}`),
	}); err != nil {
		t.Fatal(err)
	}

}

type testConsumer struct{}

func (testConsumer) Queue() string { return "test-queue" }

func (testConsumer) HandleMessage(context.Context, *Message) error { return nil }

func TestClientRunRequiresContext(t *testing.T) {
	client, err := NewClient(ClientOptions{URL: "amqps://example"})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Register(testConsumer{}); err != nil {
		t.Fatal(err)
	}
	if err := client.Run(nil); err == nil {
		t.Fatal("expected nil context error")
	}
}

func TestClientRunWithCanceledContext(t *testing.T) {
	client, err := NewClient(ClientOptions{URL: "amqps://example"})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Register(testConsumer{}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		done <- client.Run(ctx)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected run error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run should return quickly after context cancellation")
	}
}
