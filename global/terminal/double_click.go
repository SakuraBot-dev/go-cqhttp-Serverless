// +build !windows

package terminal

// RunningByDoubleClick 检查是否通过双击直接运行
func RunningByDoubleClick() bool {
	return false
}
