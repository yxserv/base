package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yxserv/base/pkg/logger"
)

// Claims JWT声明结构 - 保持与原有接口兼容
type Claims struct {
	UserID    uint64    `json:"user_id"`
	UserName  string    `json:"user_name"`
	CorpID    string    `json:"corp_id"`
	Runtime   string    `json:"runtime"`
	ExpiresAt time.Time `json:"exp"`
	IssuedAt  time.Time `json:"iat"`
	NotBefore time.Time `json:"nbf"`
	Subject   string    `json:"sub"`
}

// AuthClient 远程认证客户端
type AuthClient struct {
	// TODO: 添加gRPC客户端连接
	// grpcClient auth_pb.AuthServiceClient

	initialized bool
	jwtSecret   string
	jwtExpire   time.Duration
}

var (
	// Client 全局认证客户端实例
	Client *AuthClient
)

// Init 初始化远程认证客户端
func Init(jwtSecret string, jwtExpire time.Duration) error {
	Client = &AuthClient{
		initialized: true,
		jwtSecret:   jwtSecret,
		jwtExpire:   jwtExpire,
	}

	// TODO: 这里应该初始化gRPC连接到远程认证服务
	// conn, err := grpc.Dial(authServiceAddr, grpc.WithInsecure())
	// if err != nil {
	//     return fmt.Errorf("连接认证服务失败: %v", err)
	// }
	// Client.grpcClient = auth_pb.NewAuthServiceClient(conn)

	// 测试连接
	if err := Client.ping(); err != nil {
		return fmt.Errorf("认证服务连接测试失败: %v", err)
	}

	logger.Info("远程认证客户端初始化成功")
	return nil
}

// ping 测试认证服务连接
func (c *AuthClient) ping() error {
	// TODO: 调用远程认证服务的Ping方法
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	//
	// _, err := c.grpcClient.Ping(ctx, &auth_pb.PingRequest{})
	// return err

	// 临时实现：模拟成功
	logger.Info("认证服务连接测试成功")
	return nil
}

// GenerateToken 生成JWT Token - 保持与原有接口兼容
func GenerateToken(userID uint, userName string, corpID string, runtime string) (string, error) {
	if Client == nil {
		return "", fmt.Errorf("认证客户端未初始化")
	}

	// TODO: 调用远程认证服务的GenerateToken方法
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	//
	// req := &auth_pb.GenerateTokenRequest{
	//     UserId:   uint64(userID),
	//     UserName: userName,
	//     CorpId:   corpID,
	//     Runtime:  runtime,
	// }
	//
	// resp, err := Client.grpcClient.GenerateToken(ctx, req)
	// if err != nil {
	//     return "", err
	// }
	//
	// return resp.Token, nil

	// 临时实现：返回模拟Token（后续改为远程调用）
	tokenString := fmt.Sprintf("mock_token_%d_%s_%s_%d", userID, userName, corpID, time.Now().Unix())

	logger.Info("生成Token成功",
		logger.String("user_id", fmt.Sprintf("%d", userID)),
		logger.String("user_name", userName),
		logger.String("corp_id", corpID))

	return tokenString, nil
}

// ParseToken 解析JWT Token - 保持与原有接口兼容
func ParseToken(tokenString string) (*Claims, error) {
	if Client == nil {
		return nil, fmt.Errorf("认证客户端未初始化")
	}

	// TODO: 调用远程认证服务的ParseToken方法
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	//
	// req := &auth_pb.ParseTokenRequest{
	//     Token: tokenString,
	// }
	//
	// resp, err := Client.grpcClient.ParseToken(ctx, req)
	// if err != nil {
	//     return nil, err
	// }
	//
	// return &Claims{
	//     UserID:   resp.UserId,
	//     UserName: resp.UserName,
	//     CorpID:   resp.CorpId,
	//     Runtime:  resp.Runtime,
	// }, nil

	// 临时实现：模拟解析（后续改为远程调用）
	if !strings.HasPrefix(tokenString, "mock_token_") {
		return nil, errors.New("无效的Token格式")
	}

	// 模拟返回Claims
	return &Claims{
		UserID:    1,
		UserName:  "test_user",
		CorpID:    "test_corp",
		Runtime:   "test_runtime",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IssuedAt:  time.Now(),
		NotBefore: time.Now(),
		Subject:   "1",
	}, nil
}

// ValidateToken 验证JWT Token - 新增方法
func ValidateToken(tokenString string) (*Claims, error) {
	claims, err := ParseToken(tokenString)
	if err != nil {
		return nil, err
	}

	// 验证Token是否过期
	if claims.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("Token已过期")
	}

	return claims, nil
}

// RefreshToken 刷新JWT Token - 保持与原有接口兼容
func RefreshToken(oldTokenString string) (string, error) {
	if Client == nil {
		return "", fmt.Errorf("认证客户端未初始化")
	}

	// 解析旧Token（即使过期也允许解析）
	claims, err := ParseToken(oldTokenString)
	if err != nil {
		return "", errors.New("无效的Token")
	}

	// 生成新Token
	return GenerateToken(uint(claims.UserID), claims.UserName, claims.CorpID, claims.Runtime)
}

// GetToken 从请求中获取Token - 工具方法
// 注意：这个函数需要在实际使用时传入正确的参数
func GetToken(authHeader string, tokenQuery string) string {
	// 先从Header中获取
	auth := authHeader
	if auth == "" {
		// 尝试从查询参数获取
		auth = tokenQuery
	}

	// 提取Bearer Token
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:] // 去掉前缀
	}

	return auth
}

// Close 关闭认证客户端连接
func Close() error {
	if Client == nil {
		return nil
	}

	// TODO: 关闭gRPC连接
	// if Client.grpcConn != nil {
	//     return Client.grpcConn.Close()
	// }

	logger.Info("远程认证客户端已关闭")
	return nil
}
