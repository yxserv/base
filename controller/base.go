package controller

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/yxserv/base/pkg/logger"
	"github.com/yxserv/base/pkg/request"
	"github.com/yxserv/base/pkg/response"
	"github.com/yxserv/base/service"

	"github.com/cloudwego/hertz/pkg/app"
)

// BaseController 基础控制器
type BaseController[T any, S service.BaseService[T]] struct {
	Service S
}

// extractPaginationParams 提取分页参数
func extractPaginationParams(ac *app.RequestContext) (page, pageSize int, orderBy string) {
	pageStr := ac.DefaultQuery("page", "1")
	pageSizeStr := ac.Query("pagesize")
	if pageSizeStr == "" {
		pageSizeStr = ac.DefaultQuery("perPage", "10")
	}
	orderBy = ac.DefaultQuery("orderby", "")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err = strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = 10
	} else if pageSize > 100 {
		pageSize = 100
	}

	return page, pageSize, orderBy
}

// extractQueryFields 提取查询字段参数
// fieldsStr: 指定要提取的字段，如 "name,status:i,type"
// include_corp_id: 是否自动添加从JWT中获取的corp_id参数
// excludeFields: 要排除的字段列表
func extractQueryFields(ac *app.RequestContext, fieldsStr string, include_corp_id bool, excludeFields ...string) map[string]string {
	allParams := make(map[string]interface{})

	// 1. 从查询参数提取
	ac.QueryArgs().VisitAll(func(key, value []byte) {
		allParams[string(key)] = string(value)
	})

	// 2. 如果是POST/PUT/PATCH, 从JSON Body提取 (会覆盖同名查询参数)
	method := string(ac.Request.Method())
	if method == "POST" || method == "PUT" || method == "PATCH" {
		contentType := string(ac.Request.Header.ContentType())
		if strings.Contains(contentType, "application/json") {
			var bodyData map[string]interface{}
			if err := ac.BindJSON(&bodyData); err == nil {
				for k, v := range bodyData {
					allParams[k] = v
				}
			} else if err != io.EOF {
				logger.Warn("绑定JSON Body失败", logger.Error2(err))
			}
		}
	}

	// 3. 根据参数决定是否自动添加从JWT中获取的corp_id参数
	if include_corp_id {
		if corpID, exists := ac.Get("corp_id"); exists {
			if corpIDStr, ok := corpID.(string); ok && corpIDStr != "" {
				// 只有当查询参数中没有显式指定corp_id时才添加
				if _, hasCorpID := allParams["corp_id"]; !hasCorpID {
					allParams["corp_id"] = corpIDStr
				}
			}
		}
	}

	// 创建排除字段集合
	excludeMap := make(map[string]bool)
	for _, field := range excludeFields {
		excludeMap[field] = true
	}

	result := make(map[string]string)

	// 如果指定了字段列表，只提取指定字段
	if fieldsStr != "" {
		fields := strings.Split(fieldsStr, ",")
		for _, field := range fields {
			// 解析字段名和类型 (如 "status:i")
			// 支持操作符格式如 "title@>=:i"
			fieldName := ""
			fieldType := ""

			// 处理操作符 @
			if strings.Contains(field, "@") {
				parts := strings.Split(field, "@")
				fieldName = parts[0]

				// 处理类型部分（在@之后但在:之前）
				typePart := ""
				if len(parts) > 1 {
					if strings.Contains(parts[1], ":") {
						typeParts := strings.Split(parts[1], ":")
						typePart = typeParts[0] // 操作符部分
						if len(typeParts) > 1 {
							fieldType = typeParts[1] // 类型部分
						}
					} else {
						// 只有操作符没有类型
						typePart = parts[1]
					}
				}

				// 重新组合字段名，包含操作符
				if typePart != "" {
					fieldName = fieldName + "@" + typePart
				}
			} else {
				// 原有逻辑
				parts := strings.Split(strings.TrimSpace(field), ":")
				fieldName = parts[0]
				if len(parts) > 1 {
					fieldType = parts[1]
				}
			}

			// 跳过排除字段
			if excludeMap[fieldName] {
				continue
			}

			// 从参数中提取原始字段名（不含操作符）
			rawFieldName := fieldName
			if strings.Contains(rawFieldName, "@") {
				rawFieldName = strings.Split(rawFieldName, "@")[0]
			}

			if val, exists := allParams[rawFieldName]; exists && val != nil {
				// 类型转换处理
				if fieldType != "" {
					switch fieldType {
					case "i": // int
						if str, ok := val.(string); ok {
							result[fieldName] = str
						} else if num, ok := val.(float64); ok {
							result[fieldName] = strconv.Itoa(int(num))
						}
					case "b": // bool
						if str, ok := val.(string); ok {
							result[fieldName] = str
						} else if b, ok := val.(bool); ok {
							result[fieldName] = strconv.FormatBool(b)
						}
					default:
						result[fieldName] = fmt.Sprintf("%v", val)
					}
				} else {
					result[fieldName] = fmt.Sprintf("%v", val)
				}
			}
		}
	} else {
		// 没有指定字段，提取所有非排除字段
		for key, val := range allParams {
			if !excludeMap[key] && val != nil {
				result[key] = fmt.Sprintf("%v", val)
			}
		}
	}

	return result
}

// setStructFields 使用反射设置结构体字段值
func setStructFields(item interface{}, data map[string]interface{}) {
	v := reflect.ValueOf(item).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// 跳过不可设置的字段
		if !field.CanSet() {
			continue
		}

		// 获取字段的json标签作为键名
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// 处理json标签中的选项 (如 "name,omitempty")
		tagParts := strings.Split(jsonTag, ",")
		fieldName := tagParts[0]

		// 从数据中获取值
		if val, exists := data[fieldName]; exists {
			// 根据字段类型设置值
			switch field.Kind() {
			case reflect.String:
				if str, ok := val.(string); ok {
					field.SetString(str)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if intVal, ok := val.(int); ok {
					field.SetInt(int64(intVal))
				} else if str, ok := val.(string); ok {
					if intVal, err := strconv.ParseInt(str, 10, 64); err == nil {
						field.SetInt(intVal)
					}
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if uintVal, ok := val.(uint); ok {
					field.SetUint(uint64(uintVal))
				} else if intVal, ok := val.(int); ok && intVal >= 0 {
					field.SetUint(uint64(intVal))
				} else if str, ok := val.(string); ok {
					if uintVal, err := strconv.ParseUint(str, 10, 64); err == nil {
						field.SetUint(uintVal)
					}
				}
			case reflect.Bool:
				if boolVal, ok := val.(bool); ok {
					field.SetBool(boolVal)
				} else if str, ok := val.(string); ok {
					if boolVal, err := strconv.ParseBool(str); err == nil {
						field.SetBool(boolVal)
					}
				}
			case reflect.Float32, reflect.Float64:
				if floatVal, ok := val.(float64); ok {
					field.SetFloat(floatVal)
				} else if str, ok := val.(string); ok {
					if floatVal, err := strconv.ParseFloat(str, 64); err == nil {
						field.SetFloat(floatVal)
					}
				}
			}
		}
	}
}

// GetList 获取分页列表
// 用法：controller.GetList() 或 controller.GetList("name,status:i,type")
// 不传参数则查询所有字段，传参数则按指定字段查询
func (c *BaseController[T, S]) GetList(fieldsStr ...string) func(context.Context, *app.RequestContext) {
	return func(ctx context.Context, ac *app.RequestContext) {
		page, pageSize, orderBy := extractPaginationParams(ac)
		var fields map[string]string
		if len(fieldsStr) > 0 && fieldsStr[0] != "" {
			fields = extractQueryFields(ac, fieldsStr[0], true, "page", "pagesize", "pageSize", "perPage", "per_page", "orderby", "orderBy", "order_by")
		} else {
			fields = extractQueryFields(ac, "", true, "page", "pagesize", "pageSize", "perPage", "per_page", "orderby", "orderBy", "order_by")
		}

		// 调用服务层
		result, err := c.Service.GetListService(page, pageSize, fields, orderBy)
		if err != nil {
			logger.Error("获取列表失败", logger.Error2(err))
			response.ServerError(ac, err)
			return
		}

		response.SuccessPage(ac, result.Items, result.Total)
	}
}

// GetAll 获取所有记录
// 用法：controller.GetAll() 或 controller.GetAll("name,status:i,type")
// 不传参数则查询所有字段，传参数则按指定字段查询
func (c *BaseController[T, S]) GetAll(fieldsStr ...string) func(context.Context, *app.RequestContext) {
	return func(ctx context.Context, ac *app.RequestContext) {
		orderBy := ac.DefaultQuery("orderby", "")
		var fields map[string]string
		if len(fieldsStr) > 0 && fieldsStr[0] != "" {
			fields = extractQueryFields(ac, fieldsStr[0], true, "orderby", "orderBy", "order_by", "page", "pagesize", "pageSize", "perPage", "per_page")
		} else {
			fields = extractQueryFields(ac, "", true, "orderby", "orderBy", "order_by", "page", "pagesize", "pageSize", "perPage", "per_page")
		}

		// 调用服务层
		result, err := c.Service.GetAllService(fields, orderBy)
		if err != nil {
			logger.Error("获取所有记录失败", logger.Error2(err))
			response.ServerError(ac, err)
			return
		}

		response.Success(ac, result)
	}
}

// GetInfo 获取单个记录
func (c *BaseController[T, S]) GetInfo() func(context.Context, *app.RequestContext) {
	return func(ctx context.Context, ac *app.RequestContext) {
		// 获取ID参数
		idStr := ac.Query("id")
		if idStr == "" {
			response.BadRequest(ac, "ID参数不能为空")
			return
		}

		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			logger.Error("解析ID失败", logger.String("id", idStr), logger.Error2(err))
			response.BadRequest(ac, "无效的ID参数")
			return
		}

		// 调用服务层
		item, found, err := c.Service.GetInfoService(uint(id))
		if err != nil {
			logger.Error("获取记录失败", logger.Int("id", int(id)), logger.Error2(err))
			response.ServerError(ac, err)
			return
		}

		if !found {
			response.NotFound(ac, "记录不存在")
			return
		}

		response.Success(ac, item)
	}
}

// GetOne 根据查询条件获取单个记录
// 用法：controller.GetOne() 或 controller.GetOne("name,status:i,type")
// 支持多种查询条件，如：?name=test&status=1
// 支持 LIKE 查询，如：?name=%test%
func (c *BaseController[T, S]) GetOne(fieldsStr ...string) func(context.Context, *app.RequestContext) {
	return func(ctx context.Context, ac *app.RequestContext) {
		var fields map[string]string
		if len(fieldsStr) > 0 && fieldsStr[0] != "" {
			fields = extractQueryFields(ac, fieldsStr[0], true)
		} else {
			fields = extractQueryFields(ac, "", true)
		}

		// 调用服务层
		item, found, err := c.Service.GetOneService(fields)
		if err != nil {
			logger.Error("获取单个记录失败", logger.Error2(err))
			response.ServerError(ac, err)
			return
		}

		if !found {
			response.NotFound(ac, "记录不存在")
			return
		}

		response.Success(ac, item)
	}
}

// GetCount 根据查询条件获取记录数量
// 用法：controller.GetCount() 或 controller.GetCount("name,status:i,type")
// 不传参数则统计所有记录，传参数则按条件统计
func (c *BaseController[T, S]) GetCount(fieldsStr ...string) func(context.Context, *app.RequestContext) {
	return func(ctx context.Context, ac *app.RequestContext) {
		var fields map[string]string
		if len(fieldsStr) > 0 && fieldsStr[0] != "" {
			fields = extractQueryFields(ac, fieldsStr[0], true)
		} else {
			fields = extractQueryFields(ac, "", true)
		}

		// 调用服务层
		count, err := c.Service.GetCountService(fields)
		if err != nil {
			logger.Error("获取记录数量失败", logger.Error2(err))
			response.ServerError(ac, err)
			return
		}

		response.Success(ac, map[string]interface{}{
			"count": count,
		})
	}
}

// Modify 修改记录
// 用法：controller.Modify("name,email,status:i") 指定允许修改的字段
// 字段类型支持：:i 整数，:b 布尔值
// 操作符支持：@!= 不等于，@= 等于，@> 大于，@>= 大于等于，@< 小于，@<= 小于等于
func (c *BaseController[T, S]) Modify(fieldsStr ...string) func(context.Context, *app.RequestContext) {
	return func(ctx context.Context, ac *app.RequestContext) {
		var item T

		// 绑定请求数据到结构体
		if err := request.Bind(ac, &item); err != nil {
			logger.Warn("参数绑定失败", logger.Error2(err))
			response.BadRequest(ac, "参数绑定失败: "+err.Error())
			return
		}

		// 验证必填字段
		if len(fieldsStr) > 0 && fieldsStr[0] != "" {
			if err := validateRequiredFields(item); err != nil {
				logger.Warn("参数验证失败", logger.String("error", err.Error()))
				response.BadRequest(ac, err.Error())
				return
			}
		}

		// 设置指定字段的值
		var data map[string]interface{}
		if len(fieldsStr) > 0 && fieldsStr[0] != "" {
			data = map[string]interface{}{}
			fields := extractQueryFields(ac, fieldsStr[0], false, "id") // 排除id字段
			for k, v := range fields {
				data[k] = v
			}
		} else {
			data = map[string]interface{}{}
			fields := extractQueryFields(ac, "", false, "id") // 排除id字段
			for k, v := range fields {
				data[k] = v
			}
		}
		setStructFields(&item, data)

		// 调用服务层
		err := c.Service.ModifyService(&item)
		if err != nil {
			logger.Error("修改记录失败", logger.Error2(err))
			response.ServerError(ac, err)
			return
		}

		response.Success(ac, nil)
	}
}

// Delete 删除记录
func (c *BaseController[T, S]) Delete() func(context.Context, *app.RequestContext) {
	return func(ctx context.Context, ac *app.RequestContext) {
		// 获取ID参数，支持查询参数和JSON body
		idStr := ac.Query("id")

		// 如果查询参数中没有ID，尝试从JSON body中获取
		if idStr == "" {
			var reqData struct {
				ID uint `json:"id" form:"id"`
			}
			if err := ac.BindJSON(&reqData); err == nil && reqData.ID > 0 {
				idStr = fmt.Sprintf("%d", reqData.ID)
			}
		}

		if idStr == "" {
			response.BadRequest(ac, "ID参数不能为空")
			return
		}

		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			logger.Error("解析ID失败", logger.String("id", idStr), logger.Error2(err))
			response.BadRequest(ac, "无效的ID参数")
			return
		}

		// 调用服务层
		if err = c.Service.DeleteService(uint(id)); err != nil {
			logger.Error("删除记录失败", logger.Int("id", int(id)), logger.Error2(err))
			response.ServerError(ac, err)
			return
		}

		response.Success(ac, map[string]interface{}{"id": id})
	}
}

// Add 添加记录
// 用法：controller.Add("name,email,status:i") 指定允许添加的字段
// 字段类型支持：:i 整数，:b 布尔值
// 操作符支持：@!= 不等于，@= 等于，@> 大于，@>= 大于等于，@< 小于，@<= 小于等于
func (c *BaseController[T, S]) Add(fieldsStr ...string) func(context.Context, *app.RequestContext) {
	return func(ctx context.Context, ac *app.RequestContext) {
		var item T

		// 绑定请求数据到结构体
		if err := request.Bind(ac, &item); err != nil {
			logger.Warn("参数绑定失败", logger.Error2(err))
			response.BadRequest(ac, "参数绑定失败: "+err.Error())
			return
		}

		// 自动设置 corp_id（如果结构体中有这个字段且未设置）
		if corpID, exists := ac.Get("corp_id"); exists {
			if corpIDStr, ok := corpID.(string); ok && corpIDStr != "" {
				setCorpIDIfAvailable(&item, corpIDStr)
			}
		}

		// 验证必填字段
		if len(fieldsStr) > 0 && fieldsStr[0] != "" {
			if err := validateRequiredFields(item); err != nil {
				logger.Warn("参数验证失败", logger.String("error", err.Error()))
				response.BadRequest(ac, err.Error())
				return
			}
		}

		// 设置指定字段的值
		var data map[string]interface{}
		if len(fieldsStr) > 0 && fieldsStr[0] != "" {
			data = map[string]interface{}{}
			fields := extractQueryFields(ac, fieldsStr[0], false, "id") // 排除id字段
			for k, v := range fields {
				data[k] = v
			}
		} else {
			data = map[string]interface{}{}
			fields := extractQueryFields(ac, "", false, "id") // 排除id字段
			for k, v := range fields {
				data[k] = v
			}
		}
		setStructFields(&item, data)

		// 调用服务层
		id, err := c.Service.AddService(&item)
		if err != nil {
			logger.Error("添加记录失败", logger.Error2(err))
			response.ServerError(ac, err)
			return
		}

		response.Success(ac, map[string]interface{}{"id": id})
	}
}

// setCorpIDIfAvailable 尝试为结构体设置 corp_id 字段
func setCorpIDIfAvailable(item interface{}, corpID string) {
	v := reflect.ValueOf(item).Elem()

	// 查找 corp_id 或 corpID 字段
	for _, fieldName := range []string{"CorpID", "CorpId", "corp_id", "corpid"} {
		field := v.FieldByName(fieldName)
		if field.IsValid() && field.CanSet() {
			if field.Kind() == reflect.String && field.String() == "" {
				field.SetString(corpID)
				return
			}
		}
	}
}
