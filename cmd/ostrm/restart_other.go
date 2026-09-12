//go:build !linux && !darwin

package main

import "errors"

func configureRestart(p *program)     {}
func restartProcess(p *program) error { return errors.New("当前平台请使用安装包升级") }
