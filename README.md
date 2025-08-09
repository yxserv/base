# Base GRPC 通用组件包

这是一个通用的 Go 基础组件包，提供了标准化的 CRUD 操作，可以被多个项目复用。

## 功能特性

- **BaseModel**: 提供标准的数据模型基类和通用 CRUD 操作
- **BaseService**: 提供标准的服务层接口和实现
- **BaseController**: 提供标准的 HTTP 控制器基类
- **工具包**: 包含 logger、request、response 等常用工具

## 安装使用

### 1. 添加依赖

在你的项目中添加依赖：

```bash
go mod edit -require github.com/yxserv/base@latest
go mod tidy
```

### 2. 初始化数据库连接

在你的项目中初始化数据库连接：

```go
import (
    "github.com/yxserv/base/model"
    "github.com/yxserv/base/pkg/logger"
)

func main() {
    // 初始化日志
    logger.Init()
    
    // 初始化数据库连接
    db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    if err != nil {
        panic(err)
    }
    
    // 设置数据库连接到 base 包
    model.SetDB(db)
}
```

### 3. 定义模型

```go
import "github.com/yxserv/base/model"

type User struct {
    model.BaseModel
    Name     string `json:"name" gorm:"comment:用户名"`
    Email    string `json:"email" gorm:"comment:邮箱"`
    Status   int    `json:"status" gorm:"comment:状态"`
}
```

### 4. 定义服务

```go
import (
    "github.com/yxserv/base/service"
)

type UserService struct {
    service.BaseServiceImpl[User]
}

func NewUserService() *UserService {
    return &UserService{}
}

// 可以添加自定义业务方法
func (s *UserService) GetUserByEmail(email string) (*User, error) {
    // 自定义业务逻辑
    return nil, nil
}
```

### 5. 定义控制器

```go
import (
    "github.com/yxserv/base/controller"
)

type UserController struct {
    controller.BaseController[User, *UserService]
}

func NewUserController() *UserController {
    return &UserController{
        BaseController: controller.BaseController[User, *UserService]{
            Service: NewUserService(),
        },
    }
}
```

### 6. 注册路由

```go
func RegisterRoutes(h *server.Hertz) {
    userController := NewUserController()
    
    userGroup := h.Group("/user")
    {
        userGroup.GET("/get_ls", userController.GetList())
        userGroup.GET("/get_all", userController.GetAll())
        userGroup.GET("/get_info", userController.GetInfo())
        userGroup.POST("/post_add", userController.Add("name,email,status:i"))
        userGroup.POST("/post_modify", userController.Modify())
        userGroup.POST("/post_del", userController.Delete())
    }
}
```

## API 接口说明

### 标准 CRUD 接口

- `GET /get_ls` - 获取分页列表
  - 参数：`page`(页码), `pagesize`(每页大小), `orderby`(排序)
  - 支持字段查询和 LIKE 查询

- `GET /get_all` - 获取所有记录
  - 参数：`orderby`(排序)
  - 支持字段查询和 LIKE 查询

- `GET /get_info` - 获取单个记录
  - 参数：`id`(记录ID)

- `POST /post_add` - 添加记录
  - Body：JSON 格式的记录数据

- `POST /post_modify` - 修改记录
  - Body：JSON 格式的记录数据（需包含ID）

- `POST /post_del` - 删除记录
  - 参数：`id`(记录ID) 或 Body：`{"id": 123}`

### 响应格式

所有接口统一返回格式：

```json
{
  "status": 0,
  "msg": "操作成功",
  "doNotDisplayToast": 0,
  "data": {}
}
```

分页接口返回格式：

```json
{
  "status": 0,
  "msg": "操作成功", 
  "doNotDisplayToast": 0,
  "data": {
    "items": [],
    "total": 100
  }
}
```

## 高级功能

### 分表支持

如果你的模型需要分表，可以实现 `ShardingModel` 接口：

```go
func (u User) EnableSharding() bool {
    return true
}

func (u User) GetShardingTableName(corpID string) string {
    return model.GetShardingTableName("user", corpID)
}
```

### 自定义查询

支持灵活的字段查询和 LIKE 查询：

```bash
# 精确查询
GET /user/get_ls?name=张三&status=1

# LIKE 查询
GET /user/get_ls?name=%张%&email=%@gmail.com
```

## 开发规范

本包遵循项目开发规范 go_14.md 的要求：

- 统一的命名规范（snake_case）
- 标准的分层架构（Model-Service-Controller）
- 统一的响应格式
- 完整的错误处理和日志记录

## 版本更新

当需要更新远程包时：

```bash
go get -u github.com/yxserv/base
go mod tidy
```
