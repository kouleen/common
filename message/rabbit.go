package message

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

const retryInterval = 2 * time.Second

type rabbitProducer struct {
	url         string
	conn        *amqp091.Connection
	channel     *amqp091.Channel
	confirmChan <-chan amqp091.Confirmation
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

func newRabbitProducer(url string) (*rabbitProducer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &rabbitProducer{
		url:    url,
		ctx:    ctx,
		cancel: cancel,
	}
	if err := p.connect(); err != nil {
		cancel()
		return nil, err
	}
	go p.reconnectLoop()
	return p, nil
}

func (p *rabbitProducer) connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	conn, err := amqp091.Dial(p.url)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}
	confirmChan := ch.NotifyPublish(make(chan amqp091.Confirmation, 100))
	if p.conn != nil && !p.conn.IsClosed() {
		_ = p.channel.Close()
		_ = p.conn.Close()
	}
	p.conn = conn
	p.channel = ch
	p.confirmChan = confirmChan
	log.Println("rabbit producer connected")
	return nil
}

func (p *rabbitProducer) reconnectLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.conn.NotifyClose(make(chan *amqp091.Error)):
			log.Println("producer connection closed, reconnecting")
		}
		for {
			select {
			case <-p.ctx.Done():
				return
			default:
			}
			err := p.connect()
			if err == nil {
				break
			}
			log.Printf("producer reconnect fail: %v, wait %v", err, retryInterval)
			time.Sleep(retryInterval)
		}
	}
}

func (p *rabbitProducer) publish(ctx context.Context, exchange, routingKey string, msg *Message) error {
	p.mu.Lock()
	ch := p.channel
	p.mu.Unlock()
	if ch == nil || ch.IsClosed() {
		return errors.New("rabbit channel not ready")
	}
	pub := amqp091.Publishing{
		DeliveryMode: amqp091.Persistent,
		Body:         msg.Body,
		Headers:      msg.Headers,
	}
	if msg.TraceID != "" {
		if pub.Headers == nil {
			pub.Headers = make(map[string]interface{})
		}
		pub.Headers["trace-id"] = msg.TraceID
	}
	err := ch.PublishWithContext(ctx, exchange, routingKey, false, false, pub)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case conf := <-p.confirmChan:
		if !conf.Ack {
			return fmt.Errorf("publish nack deliveryTag=%d", conf.DeliveryTag)
		}
	}
	return nil
}

func (p *rabbitProducer) close() error {
	p.cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	return nil
}

type rabbitConsumer struct {
	url      string
	handlers map[string]ConsumerHandler
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func newRabbitConsumer(url string) *rabbitConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &rabbitConsumer{
		url:      url,
		handlers: make(map[string]ConsumerHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (c *rabbitConsumer) register(queue string, handler ConsumerHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.handlers[queue]; exists {
		return fmt.Errorf("queue %s already registered", queue)
	}
	c.handlers[queue] = handler
	return nil
}

func (c *rabbitConsumer) start(ctx context.Context) error {
	c.mu.Lock()
	queueList := make([]string, 0, len(c.handlers))
	for q := range c.handlers {
		queueList = append(queueList, q)
	}
	c.mu.Unlock()
	for _, q := range queueList {
		h := c.handlers[q]
		c.wg.Add(1)
		go c.consumeLoop(q, h)
	}
	return nil
}

func (c *rabbitConsumer) consumeLoop(queue string, handler ConsumerHandler) {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			log.Printf("consumer queue=%s exit", queue)
			return
		default:
		}
		conn, err := amqp091.Dial(c.url)
		if err != nil {
			log.Printf("consumer %s dial err: %v, retry", queue, err)
			time.Sleep(retryInterval)
			continue
		}
		ch, err := conn.Channel()
		if err != nil {
			_ = conn.Close()
			log.Printf("consumer %s create channel err: %v", queue, err)
			time.Sleep(retryInterval)
			continue
		}
		_, err = ch.QueueDeclare(queue, true, false, false, false, nil)
		if err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("queue declare %s err: %v", queue, err)
			time.Sleep(retryInterval)
			continue
		}
		_ = ch.Qos(10, 0, false)
		msgCh, err := ch.Consume(queue, "", false, false, false, false, nil)
		if err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("consume register %s err: %v", queue, err)
			time.Sleep(retryInterval)
			continue
		}
		log.Printf("queue %s consumer ready", queue)
		closed := false
	readLoop:
		for !closed {
			select {
			case <-c.ctx.Done():
				closed = true
			case d, ok := <-msgCh:
				if !ok {
					log.Printf("queue %s delivery channel closed", queue)
					closed = true
					break readLoop
				}
				msg := &Message{Body: d.Body}
				if tid, ok := d.Headers["trace-id"].(string); ok {
					msg.TraceID = tid
				}
				err := handler(context.Background(), msg)
				if err != nil {
					log.Printf("handle msg fail queue=%s err=%v", queue, err)
					_ = d.Nack(false, true)
				} else {
					_ = d.Ack(false)
				}
			}
		}
		_ = ch.Close()
		_ = conn.Close()
		time.Sleep(retryInterval)
	}
}

func (c *rabbitConsumer) close() error {
	c.cancel()
	c.wg.Wait()
	return nil
}
