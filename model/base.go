package model

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"
)

// DB 数据库连接实例，需要在使用前通过 SetDB 设置
var DB *gorm.DB

// SetDB 设置数据库连接实例
func SetDB(db *gorm.DB) {
	DB = db
}

// BaseModel 基础模型，所有模型都应该嵌入此结构
type BaseModel struct {
	ID        uint           `json:"id" gorm:"primarykey;comment:主键ID"`
	CorpID    string         `json:"corp_id" gorm:"type:varchar(64);index;comment:企业ID"`
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime;comment:更新时间"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index;comment:删除时间"`
}

// PageResult 分页结果
type PageResult[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

// GetListModel 获取分页列表
func GetListModel[T any](page, pageSize int, fields map[string]string, orderBy string) (PageResult[T], error) {
	var items []T
	var total int64

	// 构建查询
	query := DB
	query = buildQueryWithFields(query, fields)

	// 获取总数
	if err := query.Model(new(T)).Count(&total).Error; err != nil {
		return PageResult[T]{}, err
	}

	// 排序
	if orderBy != "" {
		query = query.Order(orderBy)
	}

	// 分页
	offset := (page - 1) * pageSize
	if err := query.Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		return PageResult[T]{}, err
	}

	return PageResult[T]{
		Items: items,
		Total: total,
	}, nil
}

// GetAllModel 获取所有记录
func GetAllModel[T any](fields map[string]string, orderBy string) ([]T, error) {
	var items []T

	// 构建查询
	query := DB
	query = buildQueryWithFields(query, fields)

	// 排序
	if orderBy != "" {
		query = query.Order(orderBy)
	}

	// 执行查询
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}

// buildQueryWithFields 根据字段条件构建查询
// 支持 LIKE 查询：字段值包含 % 符号时自动使用 LIKE 查询
func buildQueryWithFields(query *gorm.DB, fields map[string]string) *gorm.DB {
	for key, value := range fields {
		if value != "" {
			// 检查是否包含通配符，如果包含则使用 LIKE 查询
			if strings.Contains(value, "%") {
				query = query.Where(key+" LIKE ?", value)
			} else {
				query = query.Where(key+" = ?", value)
			}
		}
	}
	return query
}

// GetInfoModel 获取单个记录
func GetInfoModel[T any](id uint) (T, bool, error) {
	var item T
	result := DB.First(&item, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			var zero T
			return zero, false, nil
		}
		return item, false, result.Error
	}
	return item, true, nil
}

// GetOneModel 根据查询条件获取单个记录
// 支持灵活的查询条件，如：map[string]string{"name": "test", "status": "active"}
// 支持 LIKE 查询：字段值包含 % 符号时自动使用 LIKE 查询
func GetOneModel[T any](fields map[string]string) (T, bool, error) {
	var item T

	// 构建查询条件
	query := DB
	query = buildQueryWithFields(query, fields)

	// 执行查询
	result := query.First(&item)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			var zero T
			return zero, false, nil
		}
		return item, false, result.Error
	}

	return item, true, nil
}

// AddModel 添加记录
func AddModel[T any](item *T) (uint, error) {
	if err := DB.Create(item).Error; err != nil {
		return 0, err
	}

	// 反射获取ID
	value := reflect.ValueOf(item).Elem()
	idField := value.FieldByName("ID")
	if !idField.IsValid() {
		return 0, fmt.Errorf("模型缺少ID字段")
	}

	return uint(idField.Uint()), nil
}

// ModifyModel 修改记录
func ModifyModel[T any](item *T) error {
	return DB.Save(item).Error
}

// DeleteModel 删除记录
func DeleteModel[T any](id uint) error {
	return DB.Delete(new(T), id).Error
}

// GetCountModel 根据查询条件获取记录数量
// 支持灵活的查询条件，如：map[string]string{"name": "test", "status": "active"}
// 支持 LIKE 查询：字段值包含 % 符号时自动使用 LIKE 查询
func GetCountModel[T any](fields map[string]string) (int64, error) {
	var count int64

	// 构建查询条件
	query := DB
	query = buildQueryWithFields(query, fields)

	// 执行计数查询
	if err := query.Model(new(T)).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// ==================== 分表功能 ====================

// GetShardingTableName 根据企业ID计算分表表名
// 分表规则：根据企业ID的最后一个字符确定分表后缀（a-z, 0-9）
func GetShardingTableName(baseTableName, corpID string) string {
	if corpID == "" {
		return fmt.Sprintf("%s_0", baseTableName) // 默认表
	}

	// 获取企业ID的最后一个字符
	lastChar := strings.ToLower(string(corpID[len(corpID)-1]))

	// 验证字符是否有效（a-z 或 0-9）
	if (lastChar >= "a" && lastChar <= "z") || (lastChar >= "0" && lastChar <= "9") {
		return fmt.Sprintf("%s_%s", baseTableName, lastChar)
	}

	// 如果最后一个字符不是有效字符，使用默认表
	return fmt.Sprintf("%s_0", baseTableName)
}

// GetAllShardingTableNames 获取指定基础表名的所有分表表名
func GetAllShardingTableNames(baseTableName string) []string {
	var tableNames []string

	// 数字分表 (0-9)
	for i := 0; i <= 9; i++ {
		tableNames = append(tableNames, fmt.Sprintf("%s_%d", baseTableName, i))
	}

	// 字母分表 (a-z)
	for c := 'a'; c <= 'z'; c++ {
		tableNames = append(tableNames, fmt.Sprintf("%s_%c", baseTableName, c))
	}

	return tableNames
}

// EnsureAllShardingTables 确保指定基础表名的所有分表都存在
func EnsureAllShardingTables(baseTableName string, model interface{}) error {
	tableNames := GetAllShardingTableNames(baseTableName)

	// 为每个分表执行迁移
	for _, tableName := range tableNames {
		if err := DB.Table(tableName).AutoMigrate(model); err != nil {
			return fmt.Errorf("迁移分表 %s 失败: %v", tableName, err)
		}
	}

	return nil
}
