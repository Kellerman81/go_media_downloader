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

func GetMoviesTable(ctx *context.Context) table.Table {

	movies := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := movies.GetDetail()
	detail.AddField("Id", "id", db.Integer)
	detail.AddField("Quality_reached", "quality_reached", db.Numeric).FieldBool("1", "0").FieldFilterable().FieldSortable()
	detail.AddField("Quality_profile", "quality_profile", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Missing", "missing", db.Numeric).FieldBool("1", "0").FieldFilterable().FieldSortable()
	detail.AddField("Listname", "listname", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Rootpath", "rootpath", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Title", "Title", db.Text).FieldJoin(types.Join{
		BaseTable: "movies",
		Field:     "dbmovie_id",
		JoinField: "id",
		Table:     "dbmovies",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Year", "Year", db.Numeric).FieldJoin(types.Join{
		BaseTable: "movies",
		Field:     "dbmovie_id",
		JoinField: "id",
		Table:     "dbmovies",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("IMDB", "imdb_id", db.Text).FieldJoin(types.Join{
		BaseTable: "movies",
		Field:     "dbmovie_id",
		JoinField: "id",
		Table:     "dbmovies",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddColumnButtons("Details", types.GetColumnButton("Files", icon.Info,
		action.PopUpWithIframe("/admin/info/movie_files", "see more", action.IframeData{Src: "/admin/info/movie_files", AddParameterFn: func(ctx *context.Context) string {
			return "&movie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Histories", icon.Info,
		action.PopUpWithIframe("/admin/info/movie_histories", "see more", action.IframeData{Src: "/admin/info/movie_histories", AddParameterFn: func(ctx *context.Context) string {
			return "&movie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")))
	detail.SetTable("movies").SetTitle("Movies").SetDescription("Movies")

	info := movies.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Title", "Title", db.Text).FieldJoin(types.Join{
		BaseTable: "movies",
		Field:     "dbmovie_id",
		JoinField: "id",
		Table:     "dbmovies",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Year", "Year", db.Numeric).FieldJoin(types.Join{
		BaseTable: "movies",
		Field:     "dbmovie_id",
		JoinField: "id",
		Table:     "dbmovies",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("IMDB", "imdb_id", db.Text).FieldJoin(types.Join{
		BaseTable: "movies",
		Field:     "dbmovie_id",
		JoinField: "id",
		Table:     "dbmovies",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Listname", "listname", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("movies", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Quality_reached", "quality_reached", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Quality_profile", "quality_profile", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("movies", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Missing", "missing", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Rootpath", "rootpath", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Dbmovie_id", "dbmovie_id", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
	// 	return template.Default().
	// 		Link().
	// 		SetURL("/admin/info/dbmovies/detail?__goadmin_detail_pk=" + value.Value).
	// 		SetContent(template2.HTML(value.Value)).
	// 		OpenInNewTab().
	// 		SetTabTitle(template.HTML("Movie Detail(" + value.Value + ")")).
	// 		GetContent()
	// }).FieldFilterable().FieldSortable()

	info.AddColumnButtons("Details", types.GetActionIconButton(icon.File,
		action.PopUpWithIframe("/admin/info/movie_files", "see more", action.IframeData{Src: "/admin/info/movie_files", AddParameterFn: func(ctx *context.Context) string {
			return "&movie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.History,
		action.PopUpWithIframe("/admin/info/movie_histories", "see more", action.IframeData{Src: "/admin/info/movie_histories", AddParameterFn: func(ctx *context.Context) string {
			return "&movie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.Search,
		MyPopUpWithIframe("/search", "see more", action.IframeData{Src: "/api/movies/search/id/{{.Id}}?apikey=" + config.Cfg.General.WebAPIKey}, "900px", "560px")))
	info.SetTable("movies").SetTitle("Movies").SetDescription("Movies")

	formList := movies.GetForm()
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
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).FieldOptionsFromTable("movies", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })
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
	formList.AddField("Listname", "listname", db.Text, form.SelectSingle).FieldOptionsFromTable("movies", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname").Select("listname") })
	//formList.AddField("Listname", "listname", db.Text, form.Text)
	formList.AddField("Rootpath", "rootpath", db.Text, form.Text)
	formList.AddField("Dbmovie_id", "dbmovie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbmovies", "title", "id")

	formList.SetTable("movies").SetTitle("Movies").SetDescription("Movies")

	return movies
}
