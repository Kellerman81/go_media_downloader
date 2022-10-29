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

func GetSeriesTable(ctx *context.Context) table.Table {

	series := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	info := series.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Title", "Seriename", db.Text).FieldJoin(types.Join{
		BaseTable: "series",
		Field:     "dbserie_id",
		JoinField: "id",
		Table:     "dbseries",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Listname", "listname", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Rootpath", "rootpath", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike})

	info.AddColumnButtons("Details", types.GetActionIconButton(icon.List,
		action.PopUpWithIframe("/admin/info/serie_episodes", "see more", action.IframeData{Src: "/admin/info/serie_episodes", AddParameterFn: func(ctx *context.Context) string {
			return "&serie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.File,
		action.PopUpWithIframe("/admin/info/serie_episode_files", "see more", action.IframeData{Src: "/admin/info/serie_episode_files", AddParameterFn: func(ctx *context.Context) string {
			return "&serie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.History,
		action.PopUpWithIframe("/admin/info/serie_episode_histories", "see more", action.IframeData{Src: "/admin/info/serie_episode_histories", AddParameterFn: func(ctx *context.Context) string {
			return "&serie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.Search,
		MyPopUpWithIframe("/search", "see more", action.IframeData{Src: "/api/series/search/id/{{.Id}}?apikey=" + config.Cfg.General.WebApiKey}, "900px", "560px")))

	info.AddField("Dbserie_id", "dbserie_id", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
		return template.Default().
			Link().
			SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
			GetContent()
	})
	info.AddField("Dont_upgrade", "dont_upgrade", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Dont_search", "dont_search", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Search_Specials", "search_specials", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Ignore_runtime", "ignore_runtime", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()

	info.SetTable("series").SetTitle("Series").SetDescription("Series")

	formList := series.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Listname", "listname", db.Text, form.SelectSingle).FieldOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname").Select("listname") })
	//formList.AddField("Listname", "listname", db.Text, form.Text)
	formList.AddField("Rootpath", "rootpath", db.Text, form.Text)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbseries", "seriename", "id")
	formList.AddField("Dont_upgrade", "dont_upgrade", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Dont_search", "dont_search", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Search_specials", "search_specials", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Ignore_runtime", "ignore_runtime", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})

	formList.SetTable("series").SetTitle("Series").SetDescription("Series")

	return series
}
