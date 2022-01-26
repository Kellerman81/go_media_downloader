package models

import (
	template2 "html/template"

	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template"
	"github.com/GoAdminGroup/go-admin/template/icon"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/action"
	"github.com/GoAdminGroup/go-admin/template/types/form"
	"github.com/Kellerman81/go_media_downloader/config"
)

func GetSerieEpisodesTable(ctx *context.Context) table.Table {

	serieEpisodes := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := serieEpisodes.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Lastscan", "lastscan", db.Datetime)
	detail.AddField("Blacklisted", "blacklisted", db.Numeric)
	detail.AddField("Quality_reached", "quality_reached", db.Numeric)
	detail.AddField("Quality_profile", "quality_profile", db.Text)
	detail.AddField("Missing", "missing", db.Numeric)
	detail.AddField("Dont_upgrade", "dont_upgrade", db.Numeric)
	detail.AddField("Dont_search", "dont_search", db.Numeric)
	detail.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
	detail.AddField("Serie_id", "serie_id", db.Integer)
	detail.AddField("Dbserie_id", "dbserie_id", db.Integer)
	detail.SetTable("serie_episodes").SetTitle("SerieEpisodes").SetDescription("SerieEpisodes")

	info := serieEpisodes.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	info.AddField("Title", "Seriename", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episodes",
		Field:     "dbserie_id",
		JoinField: "id",
		Table:     "dbseries",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("List", "Listname", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episodes",
		Field:     "serie_id",
		JoinField: "id",
		Table:     "series",
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable() //.FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Identifier", "Identifier", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episodes",
		Field:     "dbserie_episode_id",
		JoinField: "id",
		Table:     "dbserie_episodes",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	//info.AddField("Lastscan", "lastscan", db.Datetime)
	//info.AddField("Blacklisted", "blacklisted", db.Numeric)
	info.AddField("Quality_reached", "quality_reached", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Quality_profile", "quality_profile", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Missing", "missing", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	//info.AddField("Dont_upgrade", "dont_upgrade", db.Numeric)
	//info.AddField("Dont_search", "dont_search", db.Numeric)
	//info.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
	//info.AddField("Serie_id", "serie_id", db.Integer)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	// info.AddColumnButtons("Details", types.GetColumnButton("Files", icon.File,
	// 	action.PopUpWithIframe("/admin/info/serie_episode_files", "see more", action.IframeData{Src: "/admin/info/serie_episode_files", AddParameterFn: func(ctx *context.Context) string {
	// 		return "&serie_episode_id=" + ctx.FormValue("id")
	// 	}}, "900px", "560px")), types.GetColumnButton("Histories", icon.Info,
	// 	action.PopUpWithIframe("/admin/info/serie_episode_histories", "see more", action.IframeData{Src: "/admin/info/serie_episode_histories", AddParameterFn: func(ctx *context.Context) string {
	// 		return "&serie_episode_id=" + ctx.FormValue("id")
	// 	}}, "900px", "560px")), types.GetColumnButton("Force Scan", icon.Search,
	// 	action.PopUpWithIframe("/search", "see more", action.IframeData{Src: "/api/series/episodes/search/id/{{.Id}}?apikey=" + cfg_general.WebApiKey}, "200px", "20px")))

	info.AddColumnButtons("Details", types.GetActionIconButton(icon.File,
		action.PopUpWithIframe("/admin/info/serie_episode_files", "see more", action.IframeData{Src: "/admin/info/serie_episode_files", AddParameterFn: func(ctx *context.Context) string {
			return "&serie_episode_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.History,
		action.PopUpWithIframe("/admin/info/serie_episode_histories", "see more", action.IframeData{Src: "/admin/info/serie_episode_histories", AddParameterFn: func(ctx *context.Context) string {
			return "&serie_episode_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.Search, //action.JumpInNewTab("/api/series/episodes/search/id/{{.Id}}?apikey="+cfg_general.WebApiKey, "Search")))
		MyPopUpWithIframe("/search", "see more", action.IframeData{Src: "/api/series/episodes/search/id/{{.Id}}?apikey=" + cfg_general.WebApiKey}, "900px", "560px")))

	info.AddField("Dbserie_id", "dbserie_id", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
		return template.Default().
			Link().
			SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
			GetContent()
	})

	info.SetTable("serie_episodes").SetTitle("SerieEpisodes").SetDescription("SerieEpisodes")

	formList := serieEpisodes.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Lastscan", "lastscan", db.Datetime, form.Datetime)
	formList.AddField("Blacklisted", "blacklisted", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Quality_reached", "quality_reached", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).FieldOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })
	formList.AddField("Missing", "missing", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Dont_upgrade", "dont_upgrade", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Dont_search", "dont_search", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer, form.SelectSingle) //.FieldOptionsFromTable("dbserie_episodes", "identifier", "id", func(sql *db.SQL) *db.SQL { return sql.Where("dbserie_id", "=", value.dbserie_id) })
	formList.AddField("Serie_id", "serie_id", db.Integer, form.Number)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbseries", "seriename", "id")

	formList.SetTable("serie_episodes").SetTitle("SerieEpisodes").SetDescription("SerieEpisodes")

	return serieEpisodes
}
