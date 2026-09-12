//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func configureRestart(p *program) { p.app.Restart = func() { p.restart <- struct{}{} } }
func restartProcess(p *program) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	c := p.app.Config
	args := []string{exe, "serve", "--listen", c.Listen, "--data-dir", c.DataDir, "--strm-root", c.StrmRoot}
	if err = p.Stop(nil); err != nil {
		return fmt.Errorf("关闭服务失败，请手动重启：%w", err)
	}
	err = syscall.Exec(exe, args, os.Environ())
	// Exec errors leave the process alive; restore the saved binary and retry it.
	if old := p.app.PreviousExecutable(); old != "" {
		if restoreErr := os.Rename(old, exe); restoreErr != nil {
			return fmt.Errorf("新版启动失败：%v；恢复旧程序失败：%w", err, restoreErr)
		}
		return syscall.Exec(exe, args, os.Environ())
	}
	return err
}
