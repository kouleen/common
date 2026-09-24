package message

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
)

const (
	defaultPrefetchCount = 10
	defaultRetryInterval = 2 * time.Second
)

var (
	ErrInvalidConfig     = errors.New("message config is invalid")
	ErrConsumerStarted   = errors.New("message consumer client already started")
	ErrNoConsumer        = errors.New("no consumer registered")
	ErrNilMessage        = errors.New("message is nil")
	ErrNilConsumer       = errors.New("consumer is nil")
	ErrEmptyQueue        = errors.New("queue is empty")
	ErrEmptyRabbitURL    = errors.New("rabbitmq url is empty")
	ErrSenderNotInit     = errors.New("message sender not initialized")
	ErrProducerNotReady  = errors.New("message producer is not ready")
	defaultProducerGroup = newProducerPool()
	defaultSenderMu      sync.RWMutex
	defaultSenderURL     string
)

// Message 统一消息结构。
type Message struct {
	TraceID     string         `json:"traceId"`
	MessageID   string         `json:"messageId"`
	ContentType string         `json:"contentType"`
	Body        []byte         `json:"body"`
	Headers     map[string]any `json:"headers"`
}

// SendOptions 发送配置。
type SendOptions struct {
	URL         string
	Exchange    string
	RoutingKey  string
	Mandatory   bool
	ContentType string
}

// Consumer 只要实现该接口即可被消费客户端注册。
type Consumer interface {
	Queue() string
	HandleMessage(ctx context.Context, msg *Message) error
}

// BindingConsumer 可选扩展，声明 exchange/routing key 绑定信息。
type BindingConsumer interface {
	Consumer
	Exchange() string
	RoutingKey() string
}

// ClientOptions 消费客户端配置。
type ClientOptions struct {
	URL           string
	PrefetchCount int
	RetryInterval time.Duration
}

// Send 直接发送消息，不需要显式创建 producer 实例。
func Send(ctx context.Context, opts SendOptions, msg *Message) error {
	if err := validateSendOptions(opts); err != nil {
		return err
	}
	if msg == nil {
		return ErrNilMessage
	}
	if ctx == nil {
		ctx = context.Background()
	}

	producer, err := defaultProducerGroup.Get(opts.URL)
	if err != nil {
		return err
	}
	return producer.Publish(ctx, opts, msg)
}

// InitSender 初始化默认发送器。
func InitSender(url string) error {
	if url == "" {
		return ErrEmptyRabbitURL
	}

	defaultSenderMu.Lock()
	defaultSenderURL = url
	defaultSenderMu.Unlock()
	return nil
}

// SendToQueue 使用初始化后的默认发送器，直接向指定队列发送消息。
// RabbitMQ 下会通过默认 exchange 直接投递到同名队列。
func SendToQueue(ctx context.Context, queue string, msg *Message) error {
	if queue == "" {
		return ErrEmptyQueue
	}

	defaultSenderMu.RLock()
	url := defaultSenderURL
	defaultSenderMu.RUnlock()
	if url == "" {
		return ErrSenderNotInit
	}

	producer, err := defaultProducerGroup.Get(url)
	if err != nil {
		return err
	}
	return producer.SendToQueue(ctx, queue, msg)
}

// CloseSenders 关闭 Send 使用的默认 producer 池。
func CloseSenders() error {
	defaultSenderMu.Lock()
	defaultSenderURL = ""
	defaultSenderMu.Unlock()
	return defaultProducerGroup.Close()
}

// Client 是“实现接口即可消费”的客户端。
type Client struct {
	options   ClientOptions
	consumers map[string]Consumer
	mu        sync.Mutex
	started   bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewClient 创建消费客户端。
func NewClient(opts ClientOptions) (*Client, error) {
	if err := validateClientOptions(opts); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		options:   normalizeClientOptions(opts),
		consumers: make(map[string]Consumer),
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// Register 注册消费者；每个 queue 只能注册一个实现。
func (c *Client) Register(consumers ...Consumer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return ErrConsumerStarted
	}
	for _, consumer := range consumers {
		if consumer == nil {
			return ErrNilConsumer
		}
		queue := consumer.Queue()
		if queue == "" {
			return ErrEmptyQueue
		}
		if _, exists := c.consumers[queue]; exists {
			return fmt.Errorf("queue %s already registered", queue)
		}
		c.consumers[queue] = consumer
	}
	return nil
}

// Start 启动所有已注册的消费者。
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	if len(c.consumers) == 0 {
		c.mu.Unlock()
		return ErrNoConsumer
	}

	consumerList := make([]Consumer, 0, len(c.consumers))
	for _, consumer := range c.consumers {
		consumerList = append(consumerList, consumer)
	}
	c.started = true
	c.mu.Unlock()

	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				c.cancel()
			case <-c.ctx.Done():
			}
		}()
	}

	for _, consumer := range consumerList {
		c.wg.Add(1)
		go c.consumeLoop(consumer)
	}
	return nil
}

// Close 停止消费并等待所有后台协程退出。
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	return nil
}

type producerPool struct {
	mu        sync.Mutex
	producers map[string]*Producer
}

func newProducerPool() *producerPool {
	return &producerPool{producers: make(map[string]*Producer)}
}

func (p *producerPool) Get(url string) (*Producer, error) {
	if url == "" {
		return nil, ErrEmptyRabbitURL
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if producer, ok := p.producers[url]; ok {
		return producer, nil
	}

	producer, err := NewProducer(url)
	if err != nil {
		return nil, err
	}
	p.producers[url] = producer
	return producer, nil
}

func (p *producerPool) Close() error {
	p.mu.Lock()
	producers := make([]*Producer, 0, len(p.producers))
	for _, producer := range p.producers {
		producers = append(producers, producer)
	}
	p.producers = make(map[string]*Producer)
	p.mu.Unlock()

	var closeErr error
	for _, producer := range producers {
		if err := producer.Close(); closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

// Producer 是可复用的 RabbitMQ 发送端。
type Producer struct {
	url         string
	conn        *amqp091.Connection
	channel     *amqp091.Channel
	confirmChan <-chan amqp091.Confirmation
	notifyClose <-chan *amqp091.Error
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewProducer 创建一个可复用 producer。
func NewProducer(url string) (*Producer, error) {
	if url == "" {
		return nil, ErrEmptyRabbitURL
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := &Producer{
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

// Publish 发送消息并等待 broker confirm。
func (p *Producer) Publish(ctx context.Context, opts SendOptions, msg *Message) error {
	if err := validateSendOptions(opts); err != nil {
		return err
	}
	if msg == nil {
		return ErrNilMessage
	}
	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.channel == nil || p.channel.IsClosed() || p.confirmChan == nil {
		return ErrProducerNotReady
	}

	pub := amqp091.Publishing{
		DeliveryMode: amqp091.Persistent,
		ContentType:  firstNonEmpty(msg.ContentType, opts.ContentType, "application/octet-stream"),
		MessageId:    msg.MessageID,
		Body:         msg.Body,
		Headers:      toAMQPHeaders(msg.Headers),
	}
	if msg.TraceID != "" {
		if pub.Headers == nil {
			pub.Headers = amqp091.Table{}
		}
		pub.Headers["trace-id"] = msg.TraceID
	}

	if err := p.channel.PublishWithContext(ctx, opts.Exchange, opts.RoutingKey, opts.Mandatory, false, pub); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case conf, ok := <-p.confirmChan:
		if !ok {
			return errors.New("rabbitmq confirm channel closed")
		}
		if !conf.Ack {
			return fmt.Errorf("rabbitmq publish nack deliveryTag=%d", conf.DeliveryTag)
		}
	}
	return nil
}

// SendToQueue 直接向队列发消息。
func (p *Producer) SendToQueue(ctx context.Context, queue string, msg *Message) error {
	if queue == "" {
		return ErrEmptyQueue
	}
	if msg == nil {
		return ErrNilMessage
	}
	if ctx == nil {
		ctx = context.Background()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.channel == nil || p.channel.IsClosed() || p.confirmChan == nil {
		return ErrProducerNotReady
	}
	if _, err := p.channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}

	pub := amqp091.Publishing{
		DeliveryMode: amqp091.Persistent,
		ContentType:  firstNonEmpty(msg.ContentType, "application/octet-stream"),
		MessageId:    msg.MessageID,
		Body:         msg.Body,
		Headers:      toAMQPHeaders(msg.Headers),
	}
	if msg.TraceID != "" {
		if pub.Headers == nil {
			pub.Headers = amqp091.Table{}
		}
		pub.Headers["trace-id"] = msg.TraceID
	}

	if err := p.channel.PublishWithContext(ctx, "", queue, false, false, pub); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case conf, ok := <-p.confirmChan:
		if !ok {
			return errors.New("rabbitmq confirm channel closed")
		}
		if !conf.Ack {
			return fmt.Errorf("rabbitmq publish nack deliveryTag=%d", conf.DeliveryTag)
		}
	}
	return nil
}

// Close 关闭 producer。
func (p *Producer) Close() error {
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

func (p *Producer) connect() error {
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

	oldConn := p.conn
	oldChannel := p.channel

	p.conn = conn
	p.channel = ch
	p.confirmChan = ch.NotifyPublish(make(chan amqp091.Confirmation, 1))
	p.notifyClose = conn.NotifyClose(make(chan *amqp091.Error, 1))

	if oldChannel != nil && !oldChannel.IsClosed() {
		_ = oldChannel.Close()
	}
	if oldConn != nil && !oldConn.IsClosed() {
		_ = oldConn.Close()
	}
	return nil
}

func (p *Producer) reconnectLoop() {
	for {
		notifyClose := p.currentNotifyClose()
		select {
		case <-p.ctx.Done():
			return
		case err, ok := <-notifyClose:
			if !ok || err == nil {
				log.Println("message producer connection closed, reconnecting")
			} else {
				log.Printf("message producer connection closed: %v, reconnecting", err)
			}
		}

		for {
			select {
			case <-p.ctx.Done():
				return
			default:
			}

			if err := p.connect(); err == nil {
				break
			} else {
				log.Printf("message producer reconnect failed: %v", err)
			}

			if !sleepWithContext(p.ctx, defaultRetryInterval) {
				return
			}
		}
	}
}

func (p *Producer) currentNotifyClose() <-chan *amqp091.Error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.notifyClose
}

func (c *Client) consumeLoop(consumer Consumer) {
	defer c.wg.Done()

	queue := consumer.Queue()
	retryInterval := c.options.RetryInterval
	if retryInterval <= 0 {
		retryInterval = defaultRetryInterval
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		conn, err := amqp091.Dial(c.options.URL)
		if err != nil {
			log.Printf("message consumer dial failed queue=%s err=%v", queue, err)
			if !sleepWithContext(c.ctx, retryInterval) {
				return
			}
			continue
		}

		ch, err := conn.Channel()
		if err != nil {
			_ = conn.Close()
			log.Printf("message consumer channel failed queue=%s err=%v", queue, err)
			if !sleepWithContext(c.ctx, retryInterval) {
				return
			}
			continue
		}

		if err := c.prepareQueue(ch, consumer); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("message consumer prepare failed queue=%s err=%v", queue, err)
			if !sleepWithContext(c.ctx, retryInterval) {
				return
			}
			continue
		}

		deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
		if err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Printf("message consumer register failed queue=%s err=%v", queue, err)
			if !sleepWithContext(c.ctx, retryInterval) {
				return
			}
			continue
		}

	readLoop:
		for {
			select {
			case <-c.ctx.Done():
				break readLoop
			case delivery, ok := <-deliveries:
				if !ok {
					break readLoop
				}
				msg := &Message{
					TraceID:     headerValue(delivery.Headers, "trace-id"),
					MessageID:   delivery.MessageId,
					ContentType: delivery.ContentType,
					Body:        delivery.Body,
					Headers:     fromAMQPHeaders(delivery.Headers),
				}
				if err := consumer.HandleMessage(c.ctx, msg); err != nil {
					if nackErr := delivery.Nack(false, true); nackErr != nil {
						log.Printf("message nack failed queue=%s err=%v", queue, nackErr)
					}
					continue
				}
				if ackErr := delivery.Ack(false); ackErr != nil {
					log.Printf("message ack failed queue=%s err=%v", queue, ackErr)
				}
			}
		}

		_ = ch.Close()
		_ = conn.Close()

		if !sleepWithContext(c.ctx, retryInterval) {
			return
		}
	}
}

func (c *Client) prepareQueue(ch *amqp091.Channel, consumer Consumer) error {
	queue := consumer.Queue()
	if queue == "" {
		return ErrEmptyQueue
	}
	if err := ch.Qos(c.options.PrefetchCount, 0, false); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}

	binding, ok := consumer.(BindingConsumer)
	if !ok || binding.Exchange() == "" {
		return nil
	}

	return ch.QueueBind(queue, binding.RoutingKey(), binding.Exchange(), false, nil)
}

func validateSendOptions(opts SendOptions) error {
	if opts.URL == "" {
		return fmt.Errorf("%w: url is empty", ErrInvalidConfig)
	}
	return nil
}

func validateClientOptions(opts ClientOptions) error {
	if opts.URL == "" {
		return fmt.Errorf("%w: url is empty", ErrInvalidConfig)
	}
	return nil
}

func normalizeClientOptions(opts ClientOptions) ClientOptions {
	if opts.PrefetchCount <= 0 {
		opts.PrefetchCount = defaultPrefetchCount
	}
	if opts.RetryInterval <= 0 {
		opts.RetryInterval = defaultRetryInterval
	}
	return opts
}

func toAMQPHeaders(headers map[string]any) amqp091.Table {
	if len(headers) == 0 {
		return nil
	}
	table := make(amqp091.Table, len(headers))
	maps.Copy(table, headers)
	return table
}

func fromAMQPHeaders(headers amqp091.Table) map[string]any {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]any, len(headers))
	maps.Copy(result, headers)
	return result
}

func headerValue(headers amqp091.Table, key string) string {
	if len(headers) == 0 {
		return ""
	}
	raw, ok := headers[key]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
