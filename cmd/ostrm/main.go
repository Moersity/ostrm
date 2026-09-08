package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Moersity/ostrm/internal/app"
	"github.com/kardianos/service"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

type program struct {
	app      *app.App
	server   *http.Server
	listener net.Listener
}

func (p *program) Start(s service.Service) error { go p.server.Serve(p.listener); return nil }
func (p *program) Stop(s service.Service) error {
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	p.server.Shutdown(ctx)
	return p.app.Close()
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "OStrm:", e)
		os.Exit(1)
	}
}
func run() error {
	args := os.Args[1:]
	command := "serve"
	auto := len(args) == 0
	if len(args) > 0 && args[0][0] != '-' {
		command = args[0]
		args = args[1:]
	}
	if command == "version" {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"version": app.Version, "commit": app.Commit, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH})
	}
	file := ""
	for i, x := range args {
		if x == "--config" && i+1 < len(args) {
			file = args[i+1]
		}
	}
	c, e := app.LoadConfig(file)
	if e != nil {
		return e
	}
	f := flag.NewFlagSet("ostrm "+command, flag.ContinueOnError)
	f.StringVar(&c.Listen, "listen", c.Listen, "HTTP listen address")
	f.StringVar(&c.DataDir, "data-dir", c.DataDir, "Persistent data directory")
	f.StringVar(&c.StrmRoot, "strm-root", c.StrmRoot, "Default STRM output root")
	f.StringVar(&file, "config", file, "YAML config file")
	f.BoolVar(&c.OpenBrowser, "open-browser", auto, "Open web browser")
	portable := f.Bool("portable", false, "Store data next to executable")
	source := f.String("source", "", "Legacy data directory")
	dry := f.Bool("dry-run", false, "Preview migration")
	dest := f.String("output", "", "Backup file")
	action := ""
	if command == "service" && len(args) > 0 {
		action = args[0]
		args = args[1:]
	}
	if e = f.Parse(args); e != nil {
		return e
	}
	if *portable {
		exe, e := os.Executable()
		if e != nil {
			return e
		}
		c.DataDir = filepath.Join(filepath.Dir(exe), "data")
	}
	if command == "migrate-legacy" {
		return app.MigrateLegacy(*source, c, *dry, os.Stdout)
	}
	if command == "doctor" {
		if e = c.Resolve(); e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(c)
	}
	if command == "service" {
		if e = c.Resolve(); e != nil {
			return e
		}
		svc, e := service.New(&program{}, &service.Config{Name: "ostrm", DisplayName: "OStrm", Description: "OpenList STRM service", Arguments: []string{"serve", "--listen", c.Listen, "--data-dir", c.DataDir, "--strm-root", c.StrmRoot}})
		if e != nil {
			return e
		}
		return service.Control(svc, action)
	}
	a, e := app.New(c)
	if e != nil {
		return e
	}
	if command == "backup" {
		defer a.Close()
		if *dest == "" {
			return fmt.Errorf("需要 --output 备份文件")
		}
		return a.Store.Backup(*dest)
	}
	if command != "serve" {
		a.Close()
		return fmt.Errorf("未知命令 %s", command)
	}
	listener, e := net.Listen("tcp", a.Config.Listen)
	if e != nil {
		a.Close()
		return e
	}
	server := &http.Server{Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	p := &program{a, server, listener}
	if !service.Interactive() {
		svc, e := service.New(p, &service.Config{Name: "ostrm", DisplayName: "OStrm"})
		if e != nil {
			a.Close()
			return e
		}
		return svc.Run()
	}
	defer a.Close()
	fmt.Println("OStrm", app.Version, "http://"+listener.Addr().String())
	if c.OpenBrowser {
		go openBrowser("http://" + listener.Addr().String())
	}
	errs := make(chan error, 1)
	go func() { errs <- server.Serve(listener) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case e = <-errs:
		if e == http.ErrServerClosed {
			return nil
		}
		return e
	}
}
func openBrowser(target string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", target)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		c = exec.Command("xdg-open", target)
	}
	_ = c.Run()
}
