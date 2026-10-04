package service

import "errors"

// ErrBadRequest 表示"调用方输入有问题"（缺参数、文本为空、目标类型不支持……）。
//
// API 层靠它把这类错误翻译成 400，而不是一律 500 —— 否则使用者看到的是
// 「服务器内部错误」，实际只是自己少传了个参数（规格书 §58 的错误语义要求）。
var ErrBadRequest = errors.New("请求不合法")
