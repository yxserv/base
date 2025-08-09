package service

import "git.eykj.cn/base/grpc/model"

// BaseService 基础服务接口
type BaseService[T any] interface {
	// GetListService 获取分页列表
	GetListService(page, pageSize int, fields map[string]string, orderBy string) (model.PageResult[T], error)

	// GetAllService 获取所有记录
	GetAllService(fields map[string]string, orderBy string) ([]T, error)

	// GetInfoService 获取单个记录
	GetInfoService(id uint) (T, bool, error)

	// GetOneService 根据查询条件获取单个记录
	GetOneService(fields map[string]string) (T, bool, error)

	// GetCountService 根据查询条件获取记录数量
	GetCountService(fields map[string]string) (int64, error)

	// AddService 添加记录
	AddService(item *T) (uint, error)

	// ModifyService 修改记录
	ModifyService(item *T) error

	// DeleteService 删除记录
	DeleteService(id uint) error
}

// BaseServiceImpl 基础服务实现，所有具体服务都应嵌入此结构
type BaseServiceImpl[T any] struct{}

// GetListService 获取分页列表
func (s *BaseServiceImpl[T]) GetListService(page, pageSize int, fields map[string]string, orderBy string) (model.PageResult[T], error) {
	return model.GetListModel[T](page, pageSize, fields, orderBy)
}

// GetAllService 获取所有记录
func (s *BaseServiceImpl[T]) GetAllService(fields map[string]string, orderBy string) ([]T, error) {
	return model.GetAllModel[T](fields, orderBy)
}

// GetInfoService 获取单个记录
func (s *BaseServiceImpl[T]) GetInfoService(id uint) (T, bool, error) {
	return model.GetInfoModel[T](id)
}

// GetOneService 根据查询条件获取单个记录
func (s *BaseServiceImpl[T]) GetOneService(fields map[string]string) (T, bool, error) {
	return model.GetOneModel[T](fields)
}

// AddService 添加记录
func (s *BaseServiceImpl[T]) AddService(item *T) (uint, error) {
	return model.AddModel(item)
}

// ModifyService 修改记录
func (s *BaseServiceImpl[T]) ModifyService(item *T) error {
	return model.ModifyModel(item)
}

// DeleteService 删除记录
func (s *BaseServiceImpl[T]) DeleteService(id uint) error {
	return model.DeleteModel[T](id)
}

// GetCountService 根据查询条件获取记录数量
func (s *BaseServiceImpl[T]) GetCountService(fields map[string]string) (int64, error) {
	return model.GetCountModel[T](fields)
}
