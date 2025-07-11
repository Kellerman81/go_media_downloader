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
	return types.Panel{
		Content:         template.HTML("<h2>Some Actions you can start</h2>") + refmovie + refserie,
		Title:           "Dashboard",
		Description:     "Go Media Downloader - Dashboard",
		AutoRefresh:     true,
		RefreshInterval: []int{60},
	}, nil
}

func GetDashBoard(*context.Context) (types.Panel, error) {
	components := template2.Default()
	// colComp := components.Col()

	return types.Panel{
		Content:     components.Row().SetContent("Test").GetContent(),
		Title:       "Dashboard",
		Description: "dashboard example",
	}, nil
}
