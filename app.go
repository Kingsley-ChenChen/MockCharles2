package main

import "github.com/Kingsley-ChenChen/MockCharles2/internal/core"

// App exposes the same validated application service to the desktop UI.
type App struct{ service *core.Service }

func (a *App) Snapshot() core.Snapshot { return a.service.Snapshot() }
func (a *App) SaveConfig(config core.Config, expectedRevision int64) error {
	return a.service.SaveConfig(config, expectedRevision)
}
func (a *App) StartProxy(address string) error { return a.service.StartProxy(address) }
func (a *App) StopProxy() error                { return a.service.StopProxy() }
func (a *App) Flows() []core.Flow              { return a.service.Flows() }
func (a *App) ClearFlows()                     { a.service.ClearFlows() }
