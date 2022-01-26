package pages

import (
	"html/template"

	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/action"
	mycfg "github.com/Kellerman81/go_media_downloader/config"
)

func GetActionsPage(ctx *context.Context) (types.Panel, error) {

	cfg_general := mycfg.ConfigGet("general").Data.(mycfg.GeneralConfig)
	//components := template2.Get(config.GetTheme())
	// imdb := action.PopUpWithIframe("imdb", "IMDB Import", action.IframeData{Src: "/api/fillimdb?apikey=" + cfg_general.WebApiKey}, "200px", "200px")
	// types.NewPage(&types.NewPageParam{Iframe: true})

	refmovie := action.PopUpWithIframe("refreshmovies", "Refresh Movies Metadata", action.IframeData{Src: "/api/movies/all/refreshall?apikey=" + cfg_general.WebApiKey}, "200px", "200px").ExtContent()
	refserie := action.PopUpWithIframe("refreshseries", "Refresh Series Metadata", action.IframeData{Src: "/api/series/all/refreshall?apikey=" + cfg_general.WebApiKey}, "200px", "200px").ExtContent()
	return types.Panel{
		Content:         template.HTML("<h2>Some Actions you can start</h2>") + refmovie + refserie,
		Title:           "Dashboard",
		Description:     "Go Media Downloader - Dashboard",
		AutoRefresh:     true,
		RefreshInterval: []int{60},
	}, nil
}
