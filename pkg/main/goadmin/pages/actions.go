package pages

import (
	"html/template"

	"github.com/GoAdminGroup/go-admin/context"
	template2 "github.com/GoAdminGroup/go-admin/template"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/action"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func GetActionsPage(ctx *context.Context) (types.Panel, error) {
	// components := template2.Get(config.GetTheme())
	// imdb := action.PopUpWithIframe("imdb", "IMDB Import", action.IframeData{Src: "/api/fillimdb?apikey=" + cfg_general.WebApiKey}, "200px", "200px")
	// types.NewPage(&types.NewPageParam{Iframe: true})

	refmovie := action.PopUpWithIframe("refreshmovies", "Refresh Movies Metadata", action.IframeData{Src: "/api/movies/all/refreshall?apikey=" + config.GetSettingsGeneral().WebAPIKey}, "200px", "200px").
		ExtContent(ctx)
	refserie := action.PopUpWithIframe("refreshseries", "Refresh Series Metadata", action.IframeData{Src: "/api/series/all/refreshall?apikey=" + config.GetSettingsGeneral().WebAPIKey}, "200px", "200px").
		ExtContent(ctx)
	footer := template.HTML(`
		<div class="bg-dark text-white py-4" style="margin-top: 40px;">
			<div class="container">
				<div class="row align-items-center">
					<div class="col-md-6">
						<p class="mb-0">© 2024 Go Media Downloader - Advanced Media Automation</p>
					</div>
					<div class="col-md-6 text-md-end">
						<span class="badge bg-success me-2"><i class="fas fa-circle me-1"></i>System Online</span>
						<small class="text-muted">Web Interface v2.0</small>
					</div>
				</div>
			</div>
		</div>
	`)
	
	return types.Panel{
		Content:         template.HTML("<h2>Some Actions you can start</h2>") + refmovie + refserie + footer,
		Title:           "Dashboard",
		Description:     "Go Media Downloader - Dashboard",
		AutoRefresh:     true,
		RefreshInterval: []int{60},
	}, nil
}

func GetDashBoard(c *context.Context) (types.Panel, error) {
	components := template2.Default()
	// colComp := components.Col()

	return types.Panel{
		Content:     components.Row().SetContent("Please also check the new <a href='/api/admin' target='_blank'>webinterface</a>").GetContent(),
		Title:       "Dashboard",
		Description: "Go Media Downloader - Dashboard",
	}, nil
}
