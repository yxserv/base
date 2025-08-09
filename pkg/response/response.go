package response

import (
	"net/http"

	"git.eykj.cn/base/grpc/pkg/logger"

	"github.com/cloudwego/hertz/pkg/app"
)

// 状态码定义
const (
	StatusSuccess      = 0
	StatusBadRequest   = 400
	StatusUnauthorized = 401
	StatusForbidden    = 403
	StatusNotFound     = 404
	StatusServerError  = 500
)

// 状态码对应的消息
var statusText = map[int]string{
	StatusSuccess:      "操作成功",
	StatusBadRequest:   "请求参数错误",
	StatusUnauthorized: "未授权访问",
	StatusForbidden:    "禁止访问",
	StatusNotFound:     "资源不存在",
	StatusServerError:  "服务器内部错误",
}

// Response 统一响应结构
// 遵循 {status: 0-成功/非0-失败, msg: "提示信息", data: {}} 格式
type Response struct {
	Status            int         `json:"status"`
	Msg               string      `json:"msg"`
	DoNotDisplayToast int         `json:"doNotDisplayToast"`
	Data              interface{} `json:"data,omitempty"`
}

// PageData 分页数据结构
type PageData struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
}

// Success 成功响应 (status: 0)
// noToast: 可选参数，为 true 时前端不弹窗提示 (doNotDisplayToast: 1)
func Success(c *app.RequestContext, data interface{}, noToast ...bool) {
	toast := 0
	if len(noToast) > 0 && noToast[0] {
		toast = 1
	}
	c.JSON(http.StatusOK, Response{
		Status:            StatusSuccess,
		Msg:               statusText[StatusSuccess],
		DoNotDisplayToast: toast,
		Data:              data,
	})
}

// SuccessWithMsg 成功响应，可自定义消息 (status: 0)
// noToast: 可选参数，为 true 时前端不弹窗提示 (doNotDisplayToast: 1)
func SuccessWithMsg(c *app.RequestContext, msg string, noToast ...bool) {
	toast := 0
	if len(noToast) > 0 && noToast[0] {
		toast = 1
	}
	c.JSON(http.StatusOK, Response{
		Status:            StatusSuccess,
		Msg:               msg,
		DoNotDisplayToast: toast,
		Data:              nil,
	})
}

// SuccessPage 成功分页响应 (status: 0)
// noToast: 可选参数，为 true 时前端不弹窗提示 (doNotDisplayToast: 1)
func SuccessPage(c *app.RequestContext, items interface{}, total int64, noToast ...bool) {
	Success(c, PageData{
		Items: items,
		Total: total,
	}, noToast...)
}

// Fail 失败响应 (status: 非0)
// noToast: 可选参数，为 true 时前端不弹窗提示 (doNotDisplayToast: 1)
func Fail(c *app.RequestContext, status int, msg string, noToast ...bool) {
	// 如果 msg 为空，则根据 status 从 statusText 映射中获取
	if msg == "" {
		if val, ok := statusText[status]; ok {
			msg = val
		} else {
			msg = "未知错误"
		}
	}
	toast := 0
	if len(noToast) > 0 && noToast[0] {
		toast = 1
	}
	c.JSON(http.StatusOK, Response{
		Status:            status,
		Msg:               msg,
		DoNotDisplayToast: toast,
	})
}

// BadRequest 请求参数错误 (status: 400)
func BadRequest(c *app.RequestContext, msg string, noToast ...bool) {
	Fail(c, StatusBadRequest, msg, noToast...)
}

// Unauthorized 未授权 (status: 401)
func Unauthorized(c *app.RequestContext, msg string, noToast ...bool) {
	Fail(c, StatusUnauthorized, msg, noToast...)
}

// Forbidden 禁止访问 (status: 403)
func Forbidden(c *app.RequestContext, msg string, noToast ...bool) {
	Fail(c, StatusForbidden, msg, noToast...)
}

// NotFound 资源不存在 (status: 404)
func NotFound(c *app.RequestContext, msg string, noToast ...bool) {
	Fail(c, StatusNotFound, msg, noToast...)
}

// ServerError 服务器内部错误 (status: 500)
// 注意：此函数会记录错误日志
func ServerError(c *app.RequestContext, err error, noToast ...bool) {
	logger.Error("服务器内部错误", logger.Error2(err))
	Fail(c, StatusServerError, statusText[StatusServerError], noToast...)
}
