package errors

import (
	"fmt"
	"io"
	"runtime"
)

// I18nError 定义了上层提取国际化元数据的标准接口
type I18nError interface {
	error
	GetCode() string
	GetArgs() []any
}

// Category identifies how an application error should be presented and logged
// at a transport boundary.
type Category string

const (
	CategoryClient   Category = "client"
	CategoryAuth     Category = "auth"
	CategoryResource Category = "resource"
	CategoryDatabase Category = "database"
	CategoryNetwork  Category = "network"
)

// ClassifiedError is the transport-facing contract implemented by errors in
// this package. Callers should use errors.As so wrapped errors are recognized.
type ClassifiedError interface {
	I18nError
	Category() Category
}

// baseError 核心基础结构体，通过嵌入实现复用
type baseError struct {
	code  string    // 错误码 / i18n 模板 Key
	args  []any     // i18n 动态参数
	cause error     // 上游或根源错误
	stack []uintptr // 调用栈帧
}

// Unwrap 实现 Go 原生错误链解包接口
func (b *baseError) Unwrap() error {
	return b.cause
}

// GetCode 获取错误码
func (b *baseError) GetCode() string {
	return b.code
}

// GetArgs 获取动态参数
func (b *baseError) GetArgs() []any {
	return b.args
}

func (b *baseError) Error() string {
	if b.cause == nil {
		return b.code
	}
	return b.cause.Error()
}

// Format 实现 fmt.Formatter 接口，支持 %+v 打印完备堆栈
func (b *baseError) Format(s fmt.State, verb rune) {
	b.format(s, b.Error(), verb)
}

func (b *baseError) format(s fmt.State, message string, verb rune) {
	if verb == 'v' && s.Flag('+') {
		_, _ = io.WriteString(s, message+"\n--- Stack Trace ---\n")
		for _, pc := range b.stack {
			fn := runtime.FuncForPC(pc)
			if fn == nil {
				continue
			}
			file, line := fn.FileLine(pc)
			_, _ = fmt.Fprintf(s, "%s\n\t%s:%d\n", fn.Name(), file, line)
		}
		return
	}
	formatMessage(s, message, verb)
}

func formatMessage(s fmt.State, message string, verb rune) {
	switch verb {
	case 'v', 's':
		_, _ = io.WriteString(s, message)
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", message)
	}
}

// captureStack 全局通用的堆栈抓取工具
func captureStack(skip int) []uintptr {
	var pcs [32]uintptr
	n := runtime.Callers(skip, pcs[:])
	return pcs[0:n]
}
