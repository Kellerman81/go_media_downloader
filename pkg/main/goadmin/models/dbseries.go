package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/icon"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/action"
	"github.com/GoAdminGroup/go-admin/template/types/form"
	"github.com/Kellerman81/go_media_downloader/config"
)

func GetDbseriesTable(ctx *context.Context) table.Table {

	dbseries := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := dbseries.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	//detail.AddField("Created_at", "created_at", db.Datetime)
	//detail.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Seriename", "seriename", db.Text)
	detail.AddField("Aliases", "aliases", db.Text)
	detail.AddField("Season", "season", db.Text)
	detail.AddField("Status", "status", db.Text)
	detail.AddField("Firstaired", "firstaired", db.Text)
	detail.AddField("Network", "network", db.Text)
	detail.AddField("Runtime", "runtime", db.Text)
	detail.AddField("Language", "language", db.Text)
	detail.AddField("Genre", "genre", db.Text)
	detail.AddField("Overview", "overview", db.Text)
	detail.AddField("Rating", "rating", db.Text)
	detail.AddField("Siterating", "siterating", db.Text)
	detail.AddField("Siterating_count", "siterating_count", db.Text)
	detail.AddField("Slug", "slug", db.Text)
	detail.AddField("Imdb_id", "imdb_id", db.Text)
	detail.AddField("Thetvdb_id", "thetvdb_id", db.Integer)
	detail.AddField("Freebase_m_id", "freebase_m_id", db.Text)
	detail.AddField("Freebase_id", "freebase_id", db.Text)
	detail.AddField("Tvrage_id", "tvrage_id", db.Integer)
	detail.AddField("Facebook", "facebook", db.Text)
	detail.AddField("Instagram", "instagram", db.Text)
	detail.AddField("Twitter", "twitter", db.Text)
	detail.AddField("Banner", "banner", db.Text)
	detail.AddField("Poster", "poster", db.Text)
	detail.AddField("Fanart", "fanart", db.Text)
	detail.AddField("Identifiedby", "identifiedby", db.Text)
	detail.AddField("Trakt_id", "trakt_id", db.Integer)

	detail.SetTable("dbseries").SetTitle("Dbseries").SetDescription("Dbseries")

	info := dbseries.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Seriename", "seriename", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Aliases", "aliases", db.Text)
	// info.AddField("Season", "season", db.Text)
	info.AddField("Status", "status", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Firstaired", "firstaired", db.Text)
	// info.AddField("Network", "network", db.Text)
	// info.AddField("Runtime", "runtime", db.Text)
	// info.AddField("Language", "language", db.Text)
	info.AddField("Genre", "genre", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike})
	// info.AddField("Overview", "overview", db.Text)
	// info.AddField("Rating", "rating", db.Text)
	// info.AddField("Siterating", "siterating", db.Text)
	// info.AddField("Siterating_count", "siterating_count", db.Text)
	// info.AddField("Slug", "slug", db.Text)
	info.AddField("Imdb_id", "imdb_id", db.Text)
	info.AddField("Thetvdb_id", "thetvdb_id", db.Integer)
	// info.AddField("Freebase_m_id", "freebase_m_id", db.Text)
	// info.AddField("Freebase_id", "freebase_id", db.Text)
	// info.AddField("Tvrage_id", "tvrage_id", db.Integer)
	// info.AddField("Facebook", "facebook", db.Text)
	// info.AddField("Instagram", "instagram", db.Text)
	// info.AddField("Twitter", "twitter", db.Text)
	// info.AddField("Banner", "banner", db.Text)
	// info.AddField("Poster", "poster", db.Text)
	// info.AddField("Fanart", "fanart", db.Text)
	// info.AddField("Identifiedby", "identifiedby", db.Text)
	// info.AddField("Trakt_id", "trakt_id", db.Integer)

	info.AddColumnButtons("Details", types.GetColumnButton("Titles", icon.Info,
		action.PopUpWithIframe("/admin/info/dbserie_alternates", "see more", action.IframeData{Src: "/admin/info/dbserie_alternates", AddParameterFn: func(ctx *context.Context) string {
			return "&dbserie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Episodes", icon.Info,
		action.PopUpWithIframe("/admin/info/dbserie_episodes", "see more", action.IframeData{Src: "/admin/info/dbserie_episodes", AddParameterFn: func(ctx *context.Context) string {
			return "&dbserie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Shows", icon.Info,
		action.PopUpWithIframe("/admin/info/series", "see more", action.IframeData{Src: "/admin/info/series", AddParameterFn: func(ctx *context.Context) string {
			return "&dbserie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Refresh", icon.Refresh,
		MyPopUpWithIframe("/admin/info/refresh", "see more", action.IframeData{Src: "/api/series/refresh/{{.Id}}?apikey=" + config.SettingsGeneral.WebAPIKey}, "900px", "560px")))
	info.SetTable("dbseries").SetTitle("Dbseries").SetDescription("Dbseries")

	formList := dbseries.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Seriename", "seriename", db.Text, form.Text)
	formList.AddField("Aliases", "aliases", db.Text, form.Text)
	formList.AddField("Season", "season", db.Text, form.Text)
	formList.AddField("Status", "status", db.Text, form.Text)
	formList.AddField("Firstaired", "firstaired", db.Text, form.Text)
	formList.AddField("Network", "network", db.Text, form.Text)
	formList.AddField("Runtime", "runtime", db.Text, form.Text)
	formList.AddField("Language", "language", db.Text, form.Text)
	formList.AddField("Genre", "genre", db.Text, form.Text)
	formList.AddField("Overview", "overview", db.Text, form.RichText)
	formList.AddField("Rating", "rating", db.Text, form.Text)
	formList.AddField("Siterating", "siterating", db.Text, form.Text)
	formList.AddField("Siterating_count", "siterating_count", db.Text, form.Text)
	formList.AddField("Slug", "slug", db.Text, form.Text)
	formList.AddField("Imdb_id", "imdb_id", db.Text, form.Text)
	formList.AddField("Thetvdb_id", "thetvdb_id", db.Integer, form.Number)
	formList.AddField("Freebase_m_id", "freebase_m_id", db.Text, form.Text)
	formList.AddField("Freebase_id", "freebase_id", db.Text, form.Text)
	formList.AddField("Tvrage_id", "tvrage_id", db.Integer, form.Number)
	formList.AddField("Facebook", "facebook", db.Text, form.Text)
	formList.AddField("Instagram", "instagram", db.Text, form.Text)
	formList.AddField("Twitter", "twitter", db.Text, form.Text)
	formList.AddField("Banner", "banner", db.Text, form.Text)
	formList.AddField("Poster", "poster", db.Text, form.Text)
	formList.AddField("Fanart", "fanart", db.Text, form.Text)
	formList.AddField("Identifiedby", "identifiedby", db.Text, form.Text)
	formList.AddField("Trakt_id", "trakt_id", db.Integer, form.Number)

	formList.SetTable("dbseries").SetTitle("Dbseries").SetDescription("Dbseries")

	return dbseries
}
