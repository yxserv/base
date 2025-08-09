package controller

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// ValidationRule 验证规则结构
type ValidationRule struct {
	FieldName      string
	DisplayName    string
	Operators      []string
	ExpectedValues []string
	Required       bool
	ErrorMessage   string
}

// validateRequiredFields 验证必填字段
func validateRequiredFields(item interface{}) error {
	v := reflect.ValueOf(item).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// 获取字段的json标签
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// 处理json标签中的选项 (如 "name,omitempty")
		tagParts := strings.Split(jsonTag, ",")
		fieldName := tagParts[0]

		// 获取validate标签
		validateTag := fieldType.Tag.Get("validate")
		if validateTag != "" {
			// 检查是否为必填字段
			if strings.HasPrefix(validateTag, "*") {
				// 解析验证规则
				validationRule := parseValidationRule(validateTag[1:], fieldName) // 去掉前面的*

				// 检查字段是否满足验证规则
				if !checkValidationRule(field, validationRule) {
					return fmt.Errorf(validationRule.ErrorMessage)
				}
			}
		}
	}

	return nil
}

// parseValidationRule 解析验证规则
func parseValidationRule(validateTag, fieldName string) *ValidationRule {
	rule := &ValidationRule{
		FieldName:      fieldName,
		DisplayName:    fieldName,
		Operators:      make([]string, 0),
		ExpectedValues: make([]string, 0),
		Required:       true,
		ErrorMessage:   fmt.Sprintf("%s不能为空", fieldName),
	}

	// 处理显示名称 *fieldName|显示名称
	if strings.Contains(validateTag, "|") {
		parts := strings.Split(validateTag, "|")
		validateTag = parts[0]
		if len(parts) >= 2 && parts[1] != "" {
			rule.DisplayName = parts[1]
			rule.ErrorMessage = fmt.Sprintf("%s不能为空", parts[1])
		}
	}

	// 处理操作符 @  (如 *title@!=1 或 *score@>=60@<=100)
	if strings.Contains(validateTag, "@") {
		parts := strings.Split(validateTag, "@")
		validateTag = parts[0]

		// 处理所有的操作符部分
		for i := 1; i < len(parts); i++ {
			if parts[i] != "" {
				opParts := strings.Split(parts[i], "=")
				if len(opParts) >= 2 {
					operator := opParts[0] + "="
					expectedValue := opParts[1]
					rule.Operators = append(rule.Operators, operator)
					rule.ExpectedValues = append(rule.ExpectedValues, expectedValue)
				} else {
					rule.Operators = append(rule.Operators, parts[i])
					rule.ExpectedValues = append(rule.ExpectedValues, "")
				}
			}
		}

		if len(rule.Operators) > 0 {
			// 构建错误消息
			errorParts := make([]string, len(rule.Operators))
			for i, op := range rule.Operators {
				if rule.ExpectedValues[i] != "" {
					errorParts[i] = op + rule.ExpectedValues[i]
				} else {
					errorParts[i] = op
				}
			}
			rule.ErrorMessage = fmt.Sprintf("%s不满足条件 %s", rule.DisplayName, strings.Join(errorParts, " 且 "))
		}
	}

	// 处理模糊查询 %title% 或 *%title%
	if strings.HasPrefix(validateTag, "%") || strings.HasSuffix(validateTag, "%") {
		// 模糊查询不需要必填验证
		rule.Required = false
		rule.ErrorMessage = fmt.Sprintf("%s格式不正确", rule.DisplayName)
	}

	return rule
}

// checkValidationRule 检查字段是否满足验证规则
func checkValidationRule(field reflect.Value, rule *ValidationRule) bool {
	// 如果是模糊查询，则不需要必填验证
	if !rule.Required {
		// 对于模糊查询，检查值是否符合格式
		if field.Kind() == reflect.String {
			value := field.String()
			if value != "" {
				// 只要不为空就认为验证通过
				return true
			}
		}
		return true // 非字符串类型或空值都认为通过
	}

	// 必填验证
	if isEmptyValue(field) {
		return false
	}

	// 操作符验证 - 支持多个操作符
	for i, operator := range rule.Operators {
		if !checkOperatorCondition(field, operator, rule.ExpectedValues[i]) {
			return false
		}
	}

	return true
}

// checkOperatorCondition 检查操作符条件
func checkOperatorCondition(field reflect.Value, operator, expectedValue string) bool {
	switch field.Kind() {
	case reflect.String:
		return checkStringOperator(field.String(), operator, expectedValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return checkIntOperator(field.Int(), operator, expectedValue)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal := field.Uint()
		intVal := int64(uintVal)
		return checkIntOperator(intVal, operator, expectedValue)
	case reflect.Float32, reflect.Float64:
		floatVal := field.Float()
		expectedFloat, err := strconv.ParseFloat(expectedValue, 64)
		if err != nil {
			return false
		}
		return checkFloatOperator(floatVal, operator, expectedFloat)
	case reflect.Bool:
		boolVal := field.Bool()
		expectedBool, err := strconv.ParseBool(expectedValue)
		if err != nil {
			return false
		}
		return checkBoolOperator(boolVal, operator, expectedBool)
	}
	return false
}

// checkStringOperator 检查字符串操作符
func checkStringOperator(value, operator, expectedValue string) bool {
	switch operator {
	case "!=":
		return value != expectedValue
	case "=":
		return value == expectedValue
	default:
		return true
	}
}

// checkIntOperator 检查整数操作符
func checkIntOperator(value int64, operator, expectedValue string) bool {
	expectedInt, err := strconv.ParseInt(expectedValue, 10, 64)
	if err != nil {
		return false
	}

	switch operator {
	case "!=":
		return value != expectedInt
	case "=":
		return value == expectedInt
	case ">":
		return value > expectedInt
	case ">=":
		return value >= expectedInt
	case "<":
		return value < expectedInt
	case "<=":
		return value <= expectedInt
	default:
		return true
	}
}

// checkFloatOperator 检查浮点数操作符
func checkFloatOperator(value float64, operator string, expectedValue float64) bool {
	switch operator {
	case "!=":
		return value != expectedValue
	case "=":
		return value == expectedValue
	case ">":
		return value > expectedValue
	case ">=":
		return value >= expectedValue
	case "<":
		return value < expectedValue
	case "<=":
		return value <= expectedValue
	default:
		return true
	}
}

// checkBoolOperator 检查布尔操作符
func checkBoolOperator(value bool, operator string, expectedValue bool) bool {
	switch operator {
	case "!=":
		return value != expectedValue
	case "=":
		return value == expectedValue
	default:
		return true
	}
}

// isEmptyValue 检查值是否为空
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	}
	return false
}

// parseRequiredFieldError 解析必填字段的错误消息
func parseRequiredFieldError(validateTag, fieldName string) string {
	// 格式: *fieldName|显示名称
	if strings.Contains(validateTag, "|") {
		parts := strings.Split(validateTag[1:], "|") // 去掉前面的*
		if len(parts) >= 2 && parts[1] != "" {
			return fmt.Sprintf("%s不能为空", parts[1])
		}
	}
	// 默认错误消息
	return fmt.Sprintf("%s不能为空", fieldName)
}