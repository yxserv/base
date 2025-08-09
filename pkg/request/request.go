package request

import (
	"errors"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"

	"git.eykj.cn/base/grpc/pkg/logger"

	"github.com/cloudwego/hertz/pkg/app"
)

// 错误定义
var (
	ErrInvalidType    = errors.New("无效的目标类型，必须是指针")
	ErrInvalidRequest = errors.New("无效的请求")
	ErrEmptyRequest   = errors.New("空请求")
)

// Bind 统一参数绑定函数，根据请求方法自动处理
// GET - 绑定查询参数
// POST - 绑定JSON或表单数据
func Bind(ctx *app.RequestContext, obj interface{}) error {
	method := string(ctx.Method())

	// 处理GET请求
	if method == "GET" {
		return bindQuery(ctx, obj)
	}

	// 处理POST请求
	if method == "POST" {
		contentType := string(ctx.GetHeader("Content-Type"))

		// JSON格式
		if strings.Contains(contentType, "application/json") {
			return bindJSON(ctx, obj)
		}

		// 表单格式
		if strings.Contains(contentType, "multipart/form-data") ||
			strings.Contains(contentType, "application/x-www-form-urlencoded") {
			return bindForm(ctx, obj)
		}

		// 尝试按JSON处理
		if err := bindJSON(ctx, obj); err == nil {
			return nil
		}

		// 尝试按表单处理
		if err := bindForm(ctx, obj); err == nil {
			return nil
		}
	}

	// 请求方法不支持或内容类型无法处理
	return ErrInvalidRequest
}

// bindJSON 绑定JSON请求
func bindJSON(ctx *app.RequestContext, obj interface{}) error {
	err := ctx.BindJSON(obj)
	if err != nil {
		logger.Debug("JSON绑定失败", logger.Error2(err))
		return err
	}
	return nil
}

// bindForm 绑定表单请求
func bindForm(ctx *app.RequestContext, obj interface{}) error {
	err := ctx.BindForm(obj)
	if err != nil {
		logger.Debug("表单绑定失败", logger.Error2(err))
		return err
	}
	return nil
}

// bindQuery 绑定查询参数
func bindQuery(ctx *app.RequestContext, obj interface{}) error {
	err := ctx.BindQuery(obj)
	if err != nil {
		logger.Debug("查询参数绑定失败", logger.Error2(err))
		return err
	}
	return nil
}

// GetFile 获取上传的文件
func GetFile(ctx *app.RequestContext, name string) (*multipart.FileHeader, error) {
	file, err := ctx.FormFile(name)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// GetFiles 获取多个上传的文件
func GetFiles(ctx *app.RequestContext, name string) ([]*multipart.FileHeader, error) {
	form, err := ctx.MultipartForm()
	if err != nil {
		return nil, err
	}
	return form.File[name], nil
}

// GetRawData 获取原始请求数据
func GetRawData(ctx *app.RequestContext) []byte {
	return ctx.Request.Body()
}

// ExtractFields 从请求中提取指定字段的值
// 使用示例: fields := ExtractFields(ctx, "id", "name", "status")
func ExtractFields(ctx *app.RequestContext, fieldNames ...string) map[string]string {
	result := make(map[string]string)

	// 处理GET参数
	for _, name := range fieldNames {
		if value := ctx.Query(name); value != "" {
			result[name] = value
		}
	}

	// 处理POST参数
	for _, name := range fieldNames {
		if value := ctx.PostForm(name); value != "" {
			result[name] = value
		}
	}

	return result
}

// BuildQueryCondition 根据字段构建查询条件
// 使用示例: condition, args := BuildQueryCondition(fields)
func BuildQueryCondition(fields map[string]string) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	i := 0
	for key, value := range fields {
		if value != "" {
			if i > 0 {
				conditions = append(conditions, "AND")
			}
			conditions = append(conditions, key+" = ?")
			args = append(args, value)
			i++
		}
	}

	return strings.Join(conditions, " "), args
}


// BindFields 根据字段列表绑定数据
// 默认所有字段为字符串类型，如需其它类型在字段后添加:类型
// 支持的类型: i(int)、u(uint)、b(bool)，默认为字符串
// 例：BindFields(ctx, "name", "appid:i", "active:b")
func BindFields(ctx *app.RequestContext, fields ...string) (map[string]interface{}, error) {
	// 解析请求数据
	data := make(map[string]interface{})

	// 尝试解析为JSON
	if err := ctx.BindJSON(&data); err != nil {
		// JSON解析失败，尝试获取表单数据
		ctx.PostArgs().VisitAll(func(key, value []byte) {
			data[string(key)] = string(value)
		})

		// GET请求则获取查询参数
		if string(ctx.Method()) == "GET" {
			ctx.QueryArgs().VisitAll(func(key, value []byte) {
				data[string(key)] = string(value)
			})
		}
	}

	// 没有指定字段，返回所有数据
	if len(fields) == 0 {
		return data, nil
	}

	// 处理指定字段
	result := make(map[string]interface{})
	for _, field := range fields {
		// 解析字段名和类型
		parts := strings.Split(field, ":")
		name := parts[0]
		typ := "" // 默认为字符串
		if len(parts) > 1 {
			typ = parts[1]
		}

		// 获取字段值
		val, ok := data[name]
		if !ok {
			continue
		}

		// 类型转换
		switch typ {
		case "i": // int
			if str, ok := val.(string); ok {
				if intVal, err := strconv.Atoi(str); err == nil {
					result[name] = intVal
				}
			} else if intVal, ok := val.(float64); ok {
				result[name] = int(intVal)
			}
		case "u": // uint
			if str, ok := val.(string); ok {
				if uintVal, err := strconv.ParseUint(str, 10, 32); err == nil {
					result[name] = uint(uintVal)
				}
			} else if intVal, ok := val.(float64); ok {
				result[name] = uint(intVal)
			}
		case "b": // bool
			if str, ok := val.(string); ok {
				if boolVal, err := strconv.ParseBool(str); err == nil {
					result[name] = boolVal
				}
			} else if boolVal, ok := val.(bool); ok {
				result[name] = boolVal
			}
		default: // 字符串
			result[name] = fmt.Sprintf("%v", val)
		}
	}

	return result, nil
}
