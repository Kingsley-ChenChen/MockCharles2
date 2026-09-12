package main

import (
	"context"
	"github.com/Kingsley-ChenChen/MockCharles2/internal/certificates"
	"github.com/Kingsley-ChenChen/MockCharles2/internal/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"os"
)

// App exposes the same validated application service to the desktop UI.
type App struct {
	service *core.Service
	ctx     context.Context
}

func (a *App) startup(ctx context.Context)                 { a.ctx = ctx }
func (a *App) CertificateInfo() (certificates.Info, error) { return a.service.CertificateInfo() }
func (a *App) GenerateCertificate() (certificates.Info, error) {
	return a.service.GenerateCertificate()
}
func (a *App) ExportCertificate() (string, error) {
	der, err := a.service.PublicCertificate()
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "保存公共 CA 证书", DefaultFilename: "MockCharles-CA.crt", Filters: []runtime.FileFilter{{DisplayName: "X.509 公共证书", Pattern: "*.crt"}}})
	if err != nil || path == "" {
		return "", err
	}
	if err = os.WriteFile(path, der, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) Snapshot() core.Snapshot { return a.service.Snapshot() }
func (a *App) SaveConfig(config core.Config, expectedRevision int64) error {
	return a.service.SaveConfig(config, expectedRevision)
}
func (a *App) StartProxy(address string) error { return a.service.StartProxy(address) }
func (a *App) StopProxy() error                { return a.service.StopProxy() }
func (a *App) Flows() []core.Flow              { return a.service.Flows() }
func (a *App) ClearFlows()                     { a.service.ClearFlows() }
func (a *App) ConnectionInfo(address string) (core.ConnectionInfo, error) {
	return a.service.ConnectionInfo(address)
}
