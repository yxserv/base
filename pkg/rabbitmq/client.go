package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/yxserv/base/pkg/logger"
)

// Message 通用消息结构
type Message struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Body        []byte                 `json:"body"`
	Headers     map[string]interface{} `json:"headers"`
	RoutingKey  string                 `json:"routing_key"`
	Exchange    string                 `json:"exchange"`
	Timestamp   int64                  `json:"timestamp"`
	ContentType string                 `json:"content_type"`
	Priority    uint8                  `json:"priority"`
	Expiration  string                 `json:"expiration"`
	ReplyTo     string                 `json:"reply_to"`
	MessageID   string                 `json:"message_id"`
	UserID      string                 `json:"user_id"`
	AppID       string                 `json:"app_id"`
}

// HandlerFunc 消息处理函数类型
type HandlerFunc func(ctx context.Context, delivery amqp.Delivery) error

// RabbitMQClient RabbitMQ客户端
type RabbitMQClient struct {
	conn        *amqp.Connection
	channel     *amqp.Channel
	handlers    map[string]HandlerFunc
	handlersMu  sync.RWMutex
	consumers   map[string]*Consumer
	consumersMu sync.RWMutex
	initialized bool
	config      *Config
	mu          sync.RWMutex
}

// Consumer 消费者信息
type Consumer struct {
	QueueName   string
	RoutingKey  string
	Handler     HandlerFunc
	Cancel      context.CancelFunc
	ConsumerTag string
	IsActive    bool
}

// Config RabbitMQ配置
type Config struct {
	URL            string                 `json:"url" yaml:"url"`
	Host           string                 `json:"host" yaml:"host"`
	Port           int                    `json:"port" yaml:"port"`
	Username       string                 `json:"username" yaml:"username"`
	Password       string                 `json:"password" yaml:"password"`
	VirtualHost    string                 `json:"virtual_host" yaml:"virtual_host"`
	MaxRetries     int                    `json:"max_retries" yaml:"max_retries"`
	RetryDelay     time.Duration          `json:"retry_delay" yaml:"retry_delay"`
	ConnectTimeout time.Duration          `json:"connect_timeout" yaml:"connect_timeout"`
	HeartbeatDelay time.Duration          `json:"heartbeat_delay" yaml:"heartbeat_delay"`
	ReconnectDelay time.Duration          `json:"reconnect_delay" yaml:"reconnect_delay"`
	EnableTLS      bool                   `json:"enable_tls" yaml:"enable_tls"`
	PrefetchCount  int                    `json:"prefetch_count" yaml:"prefetch_count"`
	PrefetchSize   int                    `json:"prefetch_size" yaml:"prefetch_size"`
	Args           map[string]interface{} `json:"args" yaml:"args"`
}

// ExchangeConfig 交换机配置
type ExchangeConfig struct {
	Name       string                 `json:"name" yaml:"name"`
	Type       string                 `json:"type" yaml:"type"`
	Durable    bool                   `json:"durable" yaml:"durable"`
	AutoDelete bool                   `json:"auto_delete" yaml:"auto_delete"`
	Internal   bool                   `json:"internal" yaml:"internal"`
	NoWait     bool                   `json:"no_wait" yaml:"no_wait"`
	Args       map[string]interface{} `json:"args" yaml:"args"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	Name       string                 `json:"name" yaml:"name"`
	Durable    bool                   `json:"durable" yaml:"durable"`
	AutoDelete bool                   `json:"auto_delete" yaml:"auto_delete"`
	Exclusive  bool                   `json:"exclusive" yaml:"exclusive"`
	NoWait     bool                   `json:"no_wait" yaml:"no_wait"`
	Args       map[string]interface{} `json:"args" yaml:"args"`
}

// PublishConfig 发布配置
type PublishConfig struct {
	Exchange        string                 `json:"exchange" yaml:"exchange"`
	RoutingKey      string                 `json:"routing_key" yaml:"routing_key"`
	Mandatory       bool                   `json:"mandatory" yaml:"mandatory"`
	Immediate       bool                   `json:"immediate" yaml:"immediate"`
	ContentType     string                 `json:"content_type" yaml:"content_type"`
	ContentEncoding string                 `json:"content_encoding" yaml:"content_encoding"`
	DeliveryMode    uint8                  `json:"delivery_mode" yaml:"delivery_mode"`
	Priority        uint8                  `json:"priority" yaml:"priority"`
	CorrelationID   string                 `json:"correlation_id" yaml:"correlation_id"`
	ReplyTo         string                 `json:"reply_to" yaml:"reply_to"`
	Expiration      string                 `json:"expiration" yaml:"expiration"`
	MessageID       string                 `json:"message_id" yaml:"message_id"`
	Timestamp       time.Time              `json:"timestamp" yaml:"timestamp"`
	Type            string                 `json:"type" yaml:"type"`
	UserID          string                 `json:"user_id" yaml:"user_id"`
	AppID           string                 `json:"app_id" yaml:"app_id"`
	Headers         map[string]interface{} `json:"headers" yaml:"headers"`
}

var (
	// Client 全局RabbitMQ客户端实例 - 保持向后兼容
	Client *RabbitMQClient
)

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:           "localhost",
		Port:           5672,
		Username:       "guest",
		Password:       "guest",
		VirtualHost:    "/",
		MaxRetries:     3,
		RetryDelay:     2 * time.Second,
		ConnectTimeout: 10 * time.Second,
		HeartbeatDelay: 10 * time.Second,
		ReconnectDelay: 5 * time.Second,
		EnableTLS:      false,
		PrefetchCount:  1,
		PrefetchSize:   0,
	}
}

// BuildConnectionURL 构建连接URL
func (c *Config) BuildConnectionURL() string {
	if c.URL != "" {
		return c.URL
	}

	scheme := "amqp"
	if c.EnableTLS {
		scheme = "amqps"
	}

	return fmt.Sprintf("%s://%s:%s@%s:%d%s",
		scheme, c.Username, c.Password, c.Host, c.Port, c.VirtualHost)
}

// Connect 连接RabbitMQ - 公共方法
func (c *RabbitMQClient) Connect() error {
	return c.connect()
}

// Disconnect 断开连接 - 公共方法
func (c *RabbitMQClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 停止所有消费者
	c.consumersMu.Lock()
	for _, consumer := range c.consumers {
		if consumer.Cancel != nil {
			consumer.Cancel()
		}
	}
	c.consumers = make(map[string]*Consumer)
	c.consumersMu.Unlock()

	// 关闭通道
	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}

	// 关闭连接
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	c.initialized = false
	logger.Info("✅ RabbitMQ连接已断开")
	return nil
}

// IsConnected 检查连接状态
func (c *RabbitMQClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.conn != nil && !c.conn.IsClosed() && c.channel != nil
}

// Init 初始化RabbitMQ客户端 - 保持向后兼容
func Init() error {
	config := DefaultConfig()

	// 从环境变量获取配置
	if url := getEnv("RABBITMQ_URL", ""); url != "" {
		config.URL = url
	} else {
		config.Host = getEnv("RABBITMQ_HOST", config.Host)
		config.Username = getEnv("RABBITMQ_USERNAME", config.Username)
		config.Password = getEnv("RABBITMQ_PASSWORD", config.Password)
		config.VirtualHost = getEnv("RABBITMQ_VHOST", config.VirtualHost)
	}

	return InitWithConfig(config)
}

// InitFromGlobalConfig 从全局配置初始化 - 推荐的新方式
func InitFromGlobalConfig(globalConfig interface{}) error {
	// 这里可以接受全局配置并提取RabbitMQ相关配置
	// 具体实现取决于全局配置的结构

	// 示例：如果传入的是 config.Config 类型
	if cfg, ok := globalConfig.(interface{ GetRabbitMQConfig() *Config }); ok {
		return InitWithConfig(cfg.GetRabbitMQConfig())
	}

	// 如果没有提供配置接口，回退到环境变量方式
	return Init()
}

// InitWithConfig 使用指定配置初始化RabbitMQ客户端
func InitWithConfig(config *Config) error {
	logger.Info("🚀 开始初始化RabbitMQ客户端",
		logger.String("url", config.BuildConnectionURL()))

	Client = NewClient(config)

	// 连接RabbitMQ
	if err := Client.Connect(); err != nil {
		return fmt.Errorf("RabbitMQ连接失败: %v", err)
	}

	logger.Info("✅ RabbitMQ客户端初始化成功")
	return nil
}

// NewClient 创建新的RabbitMQ客户端实例
func NewClient(config *Config) *RabbitMQClient {
	if config == nil {
		config = DefaultConfig()
	}

	return &RabbitMQClient{
		handlers:    make(map[string]HandlerFunc),
		consumers:   make(map[string]*Consumer),
		initialized: true,
		config:      config,
	}
}

// getEnv 获取环境变量
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// connect 连接RabbitMQ
func (c *RabbitMQClient) connect() error {
	var err error
	maxRetries := c.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryDelay := c.config.RetryDelay
	if retryDelay <= 0 {
		retryDelay = time.Second * 2
	}

	connectionURL := c.config.BuildConnectionURL()
	logger.Info("🔄 正在连接RabbitMQ服务器",
		logger.String("url", connectionURL))

	// 重试连接
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			logger.Info("🔄 重试连接RabbitMQ",
				logger.Int("attempt", i+1),
				logger.Int("max_retries", maxRetries))
			time.Sleep(retryDelay)
		}

		// 建立连接
		c.conn, err = amqp.Dial(connectionURL)
		if err != nil {
			logger.Error("❌ 连接RabbitMQ失败",
				logger.String("url", connectionURL),
				logger.Int("attempt", i+1),
				logger.Error2(err))
			if i == maxRetries-1 {
				return fmt.Errorf("连接RabbitMQ失败，已重试%d次: %v", maxRetries, err)
			}
			continue
		}

		// 创建通道
		c.channel, err = c.conn.Channel()
		if err != nil {
			c.conn.Close()
			logger.Error("❌ 创建RabbitMQ通道失败",
				logger.Int("attempt", i+1),
				logger.Error2(err))
			if i == maxRetries-1 {
				return fmt.Errorf("创建RabbitMQ通道失败，已重试%d次: %v", maxRetries, err)
			}
			continue
		}

		// 设置QoS
		if c.config.PrefetchCount > 0 || c.config.PrefetchSize > 0 {
			err = c.channel.Qos(c.config.PrefetchCount, c.config.PrefetchSize, false)
			if err != nil {
				logger.Warn("⚠️ 设置QoS失败", logger.Error2(err))
			}
		}

		// 连接成功
		logger.Info("✅ RabbitMQ连接建立成功",
			logger.String("url", connectionURL),
			logger.Int("attempt", i+1))
		return nil
	}

	return fmt.Errorf("连接RabbitMQ失败，已重试%d次", maxRetries)
}

// DeclareExchange 声明交换机 - 通用方法
func (c *RabbitMQClient) DeclareExchange(config ExchangeConfig) error {
	if c.channel == nil {
		return fmt.Errorf("RabbitMQ连接未建立")
	}

	logger.Info("🔄 正在声明交换机",
		logger.String("exchange", config.Name),
		logger.String("type", config.Type),
		logger.String("durable", fmt.Sprintf("%v", config.Durable)))

	err := c.channel.ExchangeDeclare(
		config.Name,       // 交换机名称
		config.Type,       // 交换机类型
		config.Durable,    // 持久化
		config.AutoDelete, // 自动删除
		config.Internal,   // 内部使用
		config.NoWait,     // 等待服务器确认
		config.Args,       // 额外参数
	)
	if err != nil {
		logger.Error("❌ 声明交换机失败",
			logger.String("exchange", config.Name),
			logger.String("type", config.Type),
			logger.Error2(err))
		return fmt.Errorf("声明交换机失败: %v", err)
	}

	logger.Info("✅ 交换机声明成功",
		logger.String("exchange", config.Name),
		logger.String("type", config.Type))
	return nil
}

// DeclareQueue 声明队列 - 通用方法
func (c *RabbitMQClient) DeclareQueue(config QueueConfig) (*amqp.Queue, error) {
	if c.channel == nil {
		return nil, fmt.Errorf("RabbitMQ连接未建立")
	}

	logger.Info("🔄 正在声明队列",
		logger.String("queue", config.Name),
		logger.String("durable", fmt.Sprintf("%v", config.Durable)))

	queue, err := c.channel.QueueDeclare(
		config.Name,       // 队列名称
		config.Durable,    // 持久化
		config.AutoDelete, // 自动删除
		config.Exclusive,  // 独占
		config.NoWait,     // 等待服务器确认
		config.Args,       // 额外参数
	)
	if err != nil {
		logger.Error("❌ 声明队列失败",
			logger.String("queue", config.Name),
			logger.Error2(err))
		return nil, fmt.Errorf("声明队列失败: %v", err)
	}

	logger.Info("✅ 队列声明成功",
		logger.String("queue", queue.Name))
	return &queue, nil
}

// BindQueue 绑定队列到交换机 - 通用方法
func (c *RabbitMQClient) BindQueue(queueName, routingKey, exchangeName string, noWait bool, args map[string]interface{}) error {
	if c.channel == nil {
		return fmt.Errorf("RabbitMQ连接未建立")
	}

	logger.Info("🔄 正在绑定队列",
		logger.String("queue", queueName),
		logger.String("exchange", exchangeName),
		logger.String("routing_key", routingKey))

	err := c.channel.QueueBind(
		queueName,    // 队列名称
		routingKey,   // 路由键
		exchangeName, // 交换机名称
		noWait,       // 等待服务器确认
		args,         // 额外参数
	)
	if err != nil {
		logger.Error("❌ 绑定队列失败",
			logger.String("queue", queueName),
			logger.String("exchange", exchangeName),
			logger.String("routing_key", routingKey),
			logger.Error2(err))
		return fmt.Errorf("绑定队列失败: %v", err)
	}

	logger.Info("✅ 队列绑定成功",
		logger.String("queue", queueName),
		logger.String("exchange", exchangeName),
		logger.String("routing_key", routingKey))
	return nil
}

// Publish 发布消息 - 通用方法
func (c *RabbitMQClient) Publish(config PublishConfig, body []byte) error {
	if c.channel == nil {
		return fmt.Errorf("RabbitMQ连接未建立")
	}

	// 构建发布消息
	publishing := amqp.Publishing{
		ContentType:     config.ContentType,
		ContentEncoding: config.ContentEncoding,
		DeliveryMode:    config.DeliveryMode,
		Priority:        config.Priority,
		CorrelationId:   config.CorrelationID,
		ReplyTo:         config.ReplyTo,
		Expiration:      config.Expiration,
		MessageId:       config.MessageID,
		Timestamp:       config.Timestamp,
		Type:            config.Type,
		UserId:          config.UserID,
		AppId:           config.AppID,
		Headers:         config.Headers,
		Body:            body,
	}

	// 发布消息
	err := c.channel.Publish(
		config.Exchange,   // 交换机
		config.RoutingKey, // 路由键
		config.Mandatory,  // 强制发布
		config.Immediate,  // 立即发布
		publishing,
	)
	if err != nil {
		logger.Error("❌ 发布消息失败",
			logger.String("exchange", config.Exchange),
			logger.String("routing_key", config.RoutingKey),
			logger.Error2(err))
		return fmt.Errorf("发布消息失败: %v", err)
	}

	logger.Info("✅ 消息发布成功",
		logger.String("exchange", config.Exchange),
		logger.String("routing_key", config.RoutingKey),
		logger.String("message_id", config.MessageID))

	return nil
}

// PublishJSON 发布JSON消息 - 便捷方法
func (c *RabbitMQClient) PublishJSON(exchange, routingKey string, data interface{}, options ...func(*PublishConfig)) error {
	// 序列化数据
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化JSON数据失败: %v", err)
	}

	// 构建发布配置
	config := PublishConfig{
		Exchange:    exchange,
		RoutingKey:  routingKey,
		ContentType: "application/json",
		Timestamp:   time.Now(),
	}

	// 应用选项
	for _, option := range options {
		option(&config)
	}

	return c.Publish(config, body)
}

// PublishWithOptions 发布消息（带选项） - 便捷方法
func (c *RabbitMQClient) PublishWithOptions(exchange, routingKey string, body []byte, options ...func(*PublishConfig)) error {
	config := PublishConfig{
		Exchange:    exchange,
		RoutingKey:  routingKey,
		ContentType: "application/octet-stream",
		Timestamp:   time.Now(),
	}

	// 应用选项
	for _, option := range options {
		option(&config)
	}

	return c.Publish(config, body)
}

// WithContentType 设置内容类型选项
func WithContentType(contentType string) func(*PublishConfig) {
	return func(config *PublishConfig) {
		config.ContentType = contentType
	}
}

// WithHeaders 设置消息头选项
func WithHeaders(headers map[string]interface{}) func(*PublishConfig) {
	return func(config *PublishConfig) {
		config.Headers = headers
	}
}

// WithDeliveryMode 设置投递模式选项
func WithDeliveryMode(mode uint8) func(*PublishConfig) {
	return func(config *PublishConfig) {
		config.DeliveryMode = mode
	}
}

// WithPriority 设置优先级选项
func WithPriority(priority uint8) func(*PublishConfig) {
	return func(config *PublishConfig) {
		config.Priority = priority
	}
}

// WithExpiration 设置过期时间选项
func WithExpiration(expiration string) func(*PublishConfig) {
	return func(config *PublishConfig) {
		config.Expiration = expiration
	}
}

// WithMessageID 设置消息ID选项
func WithMessageID(messageID string) func(*PublishConfig) {
	return func(config *PublishConfig) {
		config.MessageID = messageID
	}
}

// RegisterHandler 注册消息处理器 - 保持向后兼容
func RegisterHandler(messageType string, handler HandlerFunc) error {
	if Client == nil {
		return fmt.Errorf("RabbitMQ客户端未初始化")
	}

	Client.handlersMu.Lock()
	defer Client.handlersMu.Unlock()

	Client.handlers[messageType] = handler
	logger.Info("注册消息处理器", logger.String("message_type", messageType))
	return nil
}

// Subscribe 订阅消息 - 通用方法
func (c *RabbitMQClient) Subscribe(queueConfig QueueConfig, routingKey, exchangeName string, handler HandlerFunc) error {
	if c.channel == nil {
		return fmt.Errorf("RabbitMQ连接未建立")
	}

	// 声明队列
	queue, err := c.DeclareQueue(queueConfig)
	if err != nil {
		return fmt.Errorf("声明队列失败: %v", err)
	}

	// 绑定队列到交换机
	err = c.BindQueue(queue.Name, routingKey, exchangeName, false, nil)
	if err != nil {
		return fmt.Errorf("绑定队列失败: %v", err)
	}

	// 开始消费消息
	msgs, err := c.channel.Consume(
		queue.Name, // 队列名称
		"",         // 消费者标签
		false,      // 手动确认
		false,      // 独占
		false,      // 不等待服务器确认
		false,      // 额外参数
		nil,        // 额外参数
	)
	if err != nil {
		return fmt.Errorf("开始消费失败: %v", err)
	}

	// 创建消费者上下文
	ctx, cancel := context.WithCancel(context.Background())
	consumer := &Consumer{
		QueueName:  queue.Name,
		RoutingKey: routingKey,
		Handler:    handler,
		Cancel:     cancel,
		IsActive:   true,
	}

	// 保存消费者信息
	c.consumersMu.Lock()
	c.consumers[queue.Name] = consumer
	c.consumersMu.Unlock()

	// 启动消息处理协程
	go c.handleMessages(ctx, msgs, queue.Name)

	logger.Info("🚀 消息消费者启动成功",
		logger.String("queue", queue.Name),
		logger.String("routing_key", routingKey),
		logger.String("exchange", exchangeName))

	return nil
}

// StartConsumer 启动消息消费者 - 保持向后兼容
func StartConsumer(queueName, routingKey string) error {
	if Client == nil {
		return fmt.Errorf("RabbitMQ客户端未初始化")
	}

	// 使用默认配置
	queueConfig := QueueConfig{
		Name:    queueName,
		Durable: true,
	}

	// 使用默认处理器
	handler := func(ctx context.Context, delivery amqp.Delivery) error {
		logger.Info("📥 收到消息",
			logger.String("routing_key", delivery.RoutingKey),
			logger.String("exchange", delivery.Exchange),
			logger.Int("body_size", len(delivery.Body)))

		// 自动确认消息
		delivery.Ack(false)
		return nil
	}

	return Client.Subscribe(queueConfig, routingKey, "nebula_exchange", handler)
}

// handleMessages 处理接收到的消息
func (c *RabbitMQClient) handleMessages(ctx context.Context, msgs <-chan amqp.Delivery, queueName string) {
	logger.Info("📨 开始监听消息", logger.String("queue", queueName))

	// 获取消费者信息
	c.consumersMu.RLock()
	consumer, exists := c.consumers[queueName]
	c.consumersMu.RUnlock()

	if !exists {
		logger.Error("❌ 消费者不存在", logger.String("queue", queueName))
		return
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("🛑 消息处理协程退出", logger.String("queue", queueName))
			return
		case delivery, ok := <-msgs:
			if !ok {
				logger.Warn("⚠️ 消息通道已关闭", logger.String("queue", queueName))
				return
			}

			// 处理消息
			c.processMessage(delivery, consumer.Handler)
		}
	}
}

// processMessage 处理单个消息
func (c *RabbitMQClient) processMessage(delivery amqp.Delivery, handler HandlerFunc) {
	logger.Info("📥 收到消息",
		logger.String("routing_key", delivery.RoutingKey),
		logger.String("exchange", delivery.Exchange),
		logger.Int("body_size", len(delivery.Body)))

	// 创建处理上下文
	ctx := context.Background()

	// 调用处理器处理消息
	if handler != nil {
		err := handler(ctx, delivery)
		if err != nil {
			logger.Error("❌ 消息处理失败",
				logger.String("routing_key", delivery.RoutingKey),
				logger.String("exchange", delivery.Exchange),
				logger.Error2(err))

			// 拒绝消息并重新入队
			delivery.Nack(false, true)
			return
		}
	}

	// 确认消息
	delivery.Ack(false)
	logger.Info("✅ 消息处理完成",
		logger.String("routing_key", delivery.RoutingKey),
		logger.String("exchange", delivery.Exchange))
}

// Unsubscribe 取消订阅
func (c *RabbitMQClient) Unsubscribe(queueName string) error {
	c.consumersMu.Lock()
	defer c.consumersMu.Unlock()

	consumer, exists := c.consumers[queueName]
	if !exists {
		return fmt.Errorf("消费者不存在: %s", queueName)
	}

	// 取消消费者
	if consumer.Cancel != nil {
		consumer.Cancel()
	}

	// 从消费者列表中移除
	delete(c.consumers, queueName)

	logger.Info("✅ 消费者已取消订阅", logger.String("queue", queueName))
	return nil
}
