package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckErr(t *testing.T) {
	// 由于 CheckErr 会退出程序，我们只能测试不触发退出的情况

	// 使用 defer 捕获退出，但这里保持简单示例
	// 实际中可以使用更复杂的测试策略
	// 这里只测试当 err 为 nil 时不发生任何操作
	assert.NotPanics(t, func() {
		CheckErr(nil, "This should not panic")
	})

	// 不能直接测试触发退出的情况
}
