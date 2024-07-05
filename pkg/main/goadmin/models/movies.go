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
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func getMoviesTable(ctx *context.Context) table.Table {
	movies := table.NewDefaultTable(ctx, table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

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
	detail.AddColumnButtons(ctx, "Details", types.GetColumnButton("Files", icon.Info,
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
	info.AddField("Listname", "listname", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("movies", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	info.AddField("Quality_reached", "quality_reached", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	info.AddField("Quality_profile", "quality_profile", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("movies", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	info.AddField("Missing", "missing", db.Numeric).FieldBool("1", "0").FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	info.AddField("Rootpath", "rootpath", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Dbmovie_id", "dbmovie_id", db.Integer).FieldDisplay(func(value types.FieldModel) any {
	// 	return template.Default().
	// 		Link().
	// 		SetURL("/admin/info/dbmovies/detail?__goadmin_detail_pk=" + value.Value).
	// 		SetContent(template2.HTML(value.Value)).
	// 		OpenInNewTab().
	// 		SetTabTitle(template.HTML("Movie Detail(" + value.Value + ")")).
	// 		GetContent()
	// }).FieldFilterable().FieldSortable()

	info.AddColumnButtons(ctx, "Details", types.GetActionIconButton(icon.File,
		action.PopUpWithIframe("/admin/info/movie_files", "see more", action.IframeData{Src: "/admin/info/movie_files", AddParameterFn: func(ctx *context.Context) string {
			return "&movie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.History,
		action.PopUpWithIframe("/admin/info/movie_histories", "see more", action.IframeData{Src: "/admin/info/movie_histories", AddParameterFn: func(ctx *context.Context) string {
			return "&movie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetActionIconButton(icon.Search,
		myPopUpWithIframe("/search", "see more", action.IframeData{Src: "/api/movies/search/id/{{.Id}}?apikey=" + config.SettingsGeneral.WebAPIKey}, "900px", "560px")))
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

func getMovieHistoriesTable(ctx *context.Context) table.Table {
	movieHistories := table.NewDefaultTable(ctx, table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := movieHistories.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	//detail.AddField("Created_at", "created_at", db.Datetime)
	//detail.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Title", "title", db.Text)
	detail.AddField("Url", "url", db.Text)
	detail.AddField("Indexer", "indexer", db.Text)
	detail.AddField("Type", "type", db.Text)
	detail.AddField("Target", "target", db.Text)
	detail.AddField("Downloaded_at", "downloaded_at", db.Datetime) //.FieldDate("YYYY-MM-dd HH:mm")
	detail.AddField("Blacklisted", "blacklisted", db.Numeric)
	detail.AddField("Quality_profile", "quality_profile", db.Text)
	detail.AddField("Resolution", "name", db.Text).FieldJoin(types.Join{
		BaseTable:  "movie_histories",
		TableAlias: "reso",
		Field:      "resolution_id",
		Table:      "qualities",
		JoinField:  "id",
	})
	detail.AddField("Quality", "name", db.Text).FieldJoin(types.Join{
		BaseTable:  "movie_histories",
		TableAlias: "qual",
		Field:      "quality_id",
		JoinField:  "id",
		Table:      "qualities",
	})
	detail.AddField("Codec", "name", db.Text).FieldJoin(types.Join{
		BaseTable:  "movie_histories",
		TableAlias: "codec",
		Field:      "codec_id",
		JoinField:  "id",
		Table:      "qualities",
	})
	detail.AddField("Audio", "name", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_histories",
		Field:     "audio_id",
		JoinField: "id",
		Table:     "qualities",
	})
	detail.AddField("Listname", "listname", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_histories",
		Field:     "movie_id",
		JoinField: "id",
		Table:     "movies",
	})
	detail.AddField("Movie Title", "title", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_histories",
		Field:     "dbmovie_id",
		Table:     "dbmovies",
		JoinField: "id",
	})
	detail.AddField("Dbmovie_id", "dbmovie_id", db.Integer).FieldDisplay(func(value types.FieldModel) any {
		return template.Default().
			Link().
			SetURL("/admin/info/dbmovies/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Movie Detail(" + value.Value + ")")).
			GetContent()
	})

	detail.SetTable("movie_histories").SetTitle("MovieHistories").SetDescription("MovieHistories")

	info := movieHistories.GetInfo().HideFilterArea()
	info.HideNewButton()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Title", "title", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Url", "url", db.Text)
	info.AddField("Indexer", "indexer", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Type", "type", db.Text)
	// info.AddField("Target", "target", db.Text)
	info.AddField("Downloaded_at", "downloaded_at", db.Datetime).FieldSortable().FieldFilterable(types.FilterType{FormType: form.DatetimeRange})
	// info.AddField("Blacklisted", "blacklisted", db.Numeric)
	info.AddField("Quality_profile", "quality_profile", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("movies", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	// info.AddField("Resolution_id", "resolution_id", db.Integer)
	// info.AddField("Quality_id", "quality_id", db.Integer)
	// info.AddField("Codec_id", "codec_id", db.Integer)
	// info.AddField("Audio_id", "audio_id", db.Integer)
	info.AddField("List", "listname", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_histories",
		Field:     "movie_id",
		JoinField: "id",
		Table:     "movies",
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("movies", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	info.AddField("Movie Title", "title", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_histories",
		Field:     "dbmovie_id",
		Table:     "dbmovies",
		JoinField: "id",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Dbmovie_id", "dbmovie_id", db.Integer).FieldDisplay(func(value types.FieldModel) any {
		return template.Default().
			Link().
			SetURL("/admin/info/dbmovies/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Movie Detail(" + value.Value + ")")).
			GetContent()
	})

	info.SetTable("movie_histories").SetTitle("MovieHistories").SetDescription("MovieHistories")

	formList := movieHistories.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Url", "url", db.Text, form.Text)
	formList.AddField("Indexer", "indexer", db.Text, form.Text)
	formList.AddField("Type", "type", db.Text, form.Text)
	formList.AddField("Target", "target", db.Text, form.Text)
	formList.AddField("Downloaded_at", "downloaded_at", db.Datetime, form.Datetime)
	formList.AddField("Blacklisted", "blacklisted", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).FieldOptionsFromTable("movies", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })

	formList.AddField("Resolution", "resolution_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "1") })
	formList.AddField("Quality", "quality_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "2") })
	formList.AddField("Codec", "codec_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "3") })
	formList.AddField("Audio", "audio_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "4") })
	//formList.AddField("Movie", "movie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("movies", "title", "movies.id", func(sql *db.SQL) *db.SQL {
	//		return sql.LeftJoin("dbmovies", "dbmovies.id", "=", "movies.dbmovie_id")
	//	})

	formList.AddField("Movie_id", "movie_id", db.Integer, form.Number)
	formList.AddField("Dbmovie_id", "dbmovie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbmovies", "title", "id")

	formList.SetTable("movie_histories").SetTitle("MovieHistories").SetDescription("MovieHistories")

	return movieHistories
}

func getMovieFilesTable(ctx *context.Context) table.Table {
	movieFiles := table.NewDefaultTable(ctx, table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := movieFiles.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	//detail.AddField("Created_at", "created_at", db.Datetime)
	//detail.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Location", "location", db.Text)
	detail.AddField("Filename", "filename", db.Text)
	detail.AddField("Extension", "extension", db.Text)
	detail.AddField("Quality_profile", "quality_profile", db.Text)
	detail.AddField("Proper", "proper", db.Numeric)
	detail.AddField("Extended", "extended", db.Numeric)
	detail.AddField("Repack", "repack", db.Numeric)
	detail.AddField("Height", "height", db.Integer)
	detail.AddField("Width", "width", db.Integer)
	detail.AddField("Resolution", "name", db.Text).FieldJoin(types.Join{
		BaseTable:  "movie_files",
		TableAlias: "reso",
		Field:      "resolution_id",
		Table:      "qualities",
		JoinField:  "id",
	}).FieldHide()
	detail.AddField("Quality", "name", db.Text).FieldJoin(types.Join{
		BaseTable:  "movie_files",
		TableAlias: "qual",
		Field:      "quality_id",
		JoinField:  "id",
		Table:      "qualities",
	})
	detail.AddField("Codec", "name", db.Text).FieldJoin(types.Join{
		BaseTable:  "movie_files",
		TableAlias: "codec",
		Field:      "codec_id",
		JoinField:  "id",
		Table:      "qualities",
	})
	detail.AddField("Audio", "name", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_files",
		Field:     "audio_id",
		JoinField: "id",
		Table:     "qualities",
	})
	detail.AddField("Listname", "listname", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_files",
		Field:     "movie_id",
		JoinField: "id",
		Table:     "movies",
	})
	detail.AddField("Movie Title", "title", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_files",
		Field:     "dbmovie_id",
		Table:     "dbmovies",
		JoinField: "id",
	})

	detail.AddField("Dbmovie_id", "dbmovie_id", db.Int).FieldDisplay(func(value types.FieldModel) any {
		return template.Default().
			Link().
			SetURL("/admin/info/dbmovies/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Movie Detail(" + value.Value + ")")).
			GetContent()
	})

	detail.SetTable("movie_files").SetTitle("MovieFiles").SetDescription("MovieFiles")

	info := movieFiles.GetInfo().HideFilterArea()
	info.HideNewButton()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Location", "location", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Filename", "filename", db.Text)
	// info.AddField("Extension", "extension", db.Text)
	info.AddField("Quality_profile", "quality_profile", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("movies", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	// info.AddField("Quality_profile", "quality_profile", db.Text)
	// info.AddField("Proper", "proper", db.Numeric)
	// info.AddField("Extended", "extended", db.Numeric)
	// info.AddField("Repack", "repack", db.Numeric)
	// info.AddField("Height", "height", db.Integer)
	// info.AddField("Width", "width", db.Integer)
	// info.AddField("Resolution", "name", db.Text).FieldJoin(types.Join{
	// 	BaseTable:  "movie_files",
	// 	TableAlias: "reso",
	// 	Field:      "resolution_id",
	// 	Table:      "qualities",
	// 	JoinField:  "id",
	// }).FieldHide()
	// info.AddField("Quality_id", "name", db.Text).FieldJoin(types.Join{
	// 	BaseTable:  "movie_files",
	// 	TableAlias: "qual",
	// 	Field:      "quality_id",
	// 	JoinField:  "id",
	// 	Table:      "qualities",
	// })
	// info.AddField("Codec_id", "name", db.Text).FieldJoin(types.Join{
	// 	BaseTable:  "movie_files",
	// 	TableAlias: "codec",
	// 	Field:      "codec_id",
	// 	JoinField:  "id",
	// 	Table:      "qualities",
	// })
	// info.AddField("Audio_id", "name", db.Text).FieldJoin(types.Join{
	// 	BaseTable: "movie_files",
	// 	Field:     "audio_id",
	// 	JoinField: "id",
	// 	Table:     "qualities",
	// })
	info.AddField("List", "listname", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_files",
		Field:     "movie_id",
		JoinField: "id",
		Table:     "movies",
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("movies", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	info.AddField("Title", "title", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_files",
		Field:     "dbmovie_id",
		Table:     "dbmovies",
		JoinField: "id",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()

	info.AddField("Dbmovie_id", "dbmovie_id", db.Int).FieldDisplay(func(value types.FieldModel) any {
		return template.Default().
			Link().
			SetURL("/admin/info/dbmovies/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Movie Detail(" + value.Value + ")")).
			GetContent()
	})

	info.SetTable("movie_files").SetTitle("MovieFiles").SetDescription("MovieFiles")

	formList := movieFiles.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Location", "location", db.Text, form.Text)
	formList.AddField("Filename", "filename", db.Text, form.Text)
	formList.AddField("Extension", "extension", db.Text, form.Text)
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).FieldOptionsFromTable("movies", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })
	formList.AddField("Proper", "proper", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Extended", "extended", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Repack", "repack", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	}) // Set default value
	formList.AddField("Height", "height", db.Integer, form.Number)
	formList.AddField("Width", "width", db.Integer, form.Number)
	formList.AddField("Resolution", "resolution_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "1") })
	formList.AddField("Quality", "quality_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "2") })
	formList.AddField("Codec", "codec_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "3") })
	formList.AddField("Audio", "audio_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "4") })
	formList.AddField("Movie_id", "movie_id", db.Integer, form.Number)
	formList.AddField("Dbmovie_id", "dbmovie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbmovies", "title", "id")

	formList.SetTable("movie_files").SetTitle("MovieFiles").SetDescription("MovieFiles")

	return movieFiles
}

func getMovieFileUnmatchedsTable(ctx *context.Context) table.Table {
	movieFileUnmatcheds := table.NewDefaultTable(ctx, table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	info := movieFileUnmatcheds.GetInfo().HideFilterArea()
	info.HideDeleteButton().HideEditButton().HideNewButton()

	info.AddField("Id", "id", db.Integer).FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Listname", "listname", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Filepath", "filepath", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Last_checked", "last_checked", db.Datetime).FieldFilterable().FieldSortable()
	info.AddField("Parsed_data", "parsed_data", db.Text)

	info.SetTable("movie_file_unmatcheds").SetTitle("MovieFileUnmatcheds").SetDescription("MovieFileUnmatcheds")

	formList := movieFileUnmatcheds.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Listname", "listname", db.Text, form.Text)
	formList.AddField("Filepath", "filepath", db.Text, form.Text)
	formList.AddField("Last_checked", "last_checked", db.Datetime, form.Datetime)
	formList.AddField("Parsed_data", "parsed_data", db.Text, form.Text)

	formList.SetTable("movie_file_unmatcheds").SetTitle("MovieFileUnmatcheds").SetDescription("MovieFileUnmatcheds")

	return movieFileUnmatcheds
}

func getDbmoviesTable(ctx *context.Context) table.Table {
	//dbmovies := table.NewDefaultTable(table.DefaultConfig().SetConnection("media"))
	//dbmovies := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))
	dbmovies := table.NewDefaultTable(ctx, table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := dbmovies.GetDetail()
	detail.AddField("Id", "id", db.Integer).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Created_at", "created_at", db.Datetime)
	detail.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Title", "title", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Year", "year", db.Integer).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Imdb_id", "imdb_id", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Adult", "adult", db.Numeric)
	detail.AddField("Budget", "budget", db.Integer)
	detail.AddField("Genres", "genres", db.Text)
	detail.AddField("Original_language", "original_language", db.Text)
	detail.AddField("Original_title", "original_title", db.Text)
	detail.AddField("Overview", "overview", db.Text)
	detail.AddField("Popularity", "popularity", db.Real)
	detail.AddField("Revenue", "revenue", db.Integer)
	detail.AddField("Runtime", "runtime", db.Integer)
	detail.AddField("Spoken_languages", "spoken_languages", db.Text)
	detail.AddField("Status", "status", db.Text)
	detail.AddField("Tagline", "tagline", db.Text)
	detail.AddField("Vote_average", "vote_average", db.Real).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	detail.AddField("Vote_count", "vote_count", db.Integer).FieldSortable()
	detail.AddField("Moviedb_id", "moviedb_id", db.Integer)
	detail.AddField("Freebase_m_id", "freebase_m_id", db.Text)
	detail.AddField("Freebase_id", "freebase_id", db.Text)
	detail.AddField("Facebook_id", "facebook_id", db.Text)
	detail.AddField("Instagram_id", "instagram_id", db.Text)
	detail.AddField("Twitter_id", "twitter_id", db.Text)
	detail.AddField("Url", "url", db.Text)
	detail.AddField("Backdrop", "backdrop", db.Text)
	detail.AddField("Poster", "poster", db.Text)
	detail.AddField("Slug", "slug", db.Text)
	detail.AddField("Trakt_id", "trakt_id", db.Integer)
	detail.AddField("Release_date", "release_date", db.Datetime)
	detail.AddColumnButtons(ctx, "Details", types.GetColumnButton("Titles", icon.Info,
		action.PopUpWithIframe("/admin/info/dbmovie_titles", "see more", action.IframeData{Src: "/admin/info/dbmovie_titles", AddParameterFn: func(ctx *context.Context) string {
			return "&dbmovie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Movies", icon.Info,
		action.PopUpWithIframe("/admin/info/movies", "see more", action.IframeData{Src: "/admin/info/movies", AddParameterFn: func(ctx *context.Context) string {
			return "&dbmovie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")))
	detail.SetTable("dbmovies").SetTitle("Dbmovies").SetDescription("Dbmovies")

	info := dbmovies.GetInfo().HideFilterArea()
	info.AddField("Id", "id", db.Integer).FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Title", "title", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Year", "year", db.Integer).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Imdb_id", "imdb_id", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Adult", "adult", db.Numeric)
	// info.AddField("Budget", "budget", db.Integer)
	info.AddField("Genres", "genres", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Original_language", "original_language", db.Text)
	// info.AddField("Original_title", "original_title", db.Text)
	// info.AddField("Overview", "overview", db.Text)
	// info.AddField("Popularity", "popularity", db.Real)
	// info.AddField("Revenue", "revenue", db.Integer)
	// info.AddField("Runtime", "runtime", db.Integer)
	// info.AddField("Spoken_languages", "spoken_languages", db.Text)
	// info.AddField("Status", "status", db.Text)
	// info.AddField("Tagline", "tagline", db.Text)
	info.AddField("Vote_average", "vote_average", db.Real).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Vote_count", "vote_count", db.Integer).FieldSortable()
	//info.AddField("Moviedb_id", "moviedb_id", db.Integer)
	// info.AddField("Freebase_m_id", "freebase_m_id", db.Text)
	// info.AddField("Freebase_id", "freebase_id", db.Text)
	// info.AddField("Facebook_id", "facebook_id", db.Text)
	// info.AddField("Instagram_id", "instagram_id", db.Text)
	// info.AddField("Twitter_id", "twitter_id", db.Text)
	// info.AddField("Url", "url", db.Text)
	// info.AddField("Backdrop", "backdrop", db.Text)
	// info.AddField("Poster", "poster", db.Text)
	// info.AddField("Slug", "slug", db.Text)
	// info.AddField("Trakt_id", "trakt_id", db.Integer)
	// info.AddField("Release_date", "release_date", db.Datetime)

	info.AddColumnButtons(ctx, "Details", types.GetColumnButton("Titles", icon.Info,
		action.PopUpWithIframe("/admin/info/dbmovie_titles", "see more", action.IframeData{Src: "/admin/info/dbmovie_titles", AddParameterFn: func(ctx *context.Context) string {
			return "&dbmovie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Movies", icon.Info,
		action.PopUpWithIframe("/admin/info/movies", "see more", action.IframeData{Src: "/admin/info/movies", AddParameterFn: func(ctx *context.Context) string {
			return "&dbmovie_id=" + ctx.FormValue("id")
		}}, "900px", "560px")), types.GetColumnButton("Refresh", icon.Refresh,
		myPopUpWithIframe("/admin/info/refresh", "see more", action.IframeData{Src: "/api/movies/refresh/{{.Id}}?apikey=" + config.SettingsGeneral.WebAPIKey}, "900px", "560px")))
	info.SetTable("dbmovies").SetTitle("Dbmovies").SetDescription("Dbmovies")

	formList := dbmovies.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Year", "year", db.Integer, form.Number)
	formList.AddField("Adult", "adult", db.Numeric, form.Number)
	formList.AddField("Budget", "budget", db.Integer, form.Number)
	formList.AddField("Genres", "genres", db.Text, form.Text)
	formList.AddField("Original_language", "original_language", db.Text, form.Text)
	formList.AddField("Original_title", "original_title", db.Text, form.Text)
	formList.AddField("Overview", "overview", db.Text, form.RichText)
	formList.AddField("Popularity", "popularity", db.Real, form.Text)
	formList.AddField("Revenue", "revenue", db.Integer, form.Number)
	formList.AddField("Runtime", "runtime", db.Integer, form.Number)
	formList.AddField("Spoken_languages", "spoken_languages", db.Text, form.Text)
	formList.AddField("Status", "status", db.Text, form.Text)
	formList.AddField("Tagline", "tagline", db.Text, form.RichText)
	formList.AddField("Vote_average", "vote_average", db.Real, form.Text)
	formList.AddField("Vote_count", "vote_count", db.Integer, form.Number)
	formList.AddField("Moviedb_id", "moviedb_id", db.Integer, form.Number)
	formList.AddField("Imdb_id", "imdb_id", db.Text, form.Text)
	formList.AddField("Freebase_m_id", "freebase_m_id", db.Text, form.Text)
	formList.AddField("Freebase_id", "freebase_id", db.Text, form.Text)
	formList.AddField("Facebook_id", "facebook_id", db.Text, form.Text)
	formList.AddField("Instagram_id", "instagram_id", db.Text, form.Text)
	formList.AddField("Twitter_id", "twitter_id", db.Text, form.Text)
	formList.AddField("Url", "url", db.Text, form.Text)
	formList.AddField("Backdrop", "backdrop", db.Text, form.Text)
	formList.AddField("Poster", "poster", db.Text, form.Text)
	formList.AddField("Slug", "slug", db.Text, form.Text)
	formList.AddField("Trakt_id", "trakt_id", db.Integer, form.Number)
	formList.AddField("Release_date", "release_date", db.Datetime, form.Datetime)

	formList.SetTable("dbmovies").SetTitle("Dbmovies").SetDescription("Dbmovies")

	return dbmovies
}

func getDbmovieTitlesTable(ctx *context.Context) table.Table {
	dbmovieTitles := table.NewDefaultTable(ctx, table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	info := dbmovieTitles.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).FieldSortable()
	info.AddField("Dbmovie_id", "dbmovie_id", db.Integer).FieldDisplay(func(value types.FieldModel) any {
		return template.Default().
			Link().
			SetURL("/admin/info/dbmovies/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Movie Detail(" + value.Value + ")")).
			GetContent()
	}).FieldFilterable().FieldSortable()
	info.AddField("Title", "title", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Slug", "slug", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Region", "region", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()

	info.SetTable("dbmovie_titles").SetTitle("DbmovieTitles").SetDescription("DbmovieTitles")

	formList := dbmovieTitles.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	formList.AddField("Dbmovie_id", "dbmovie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbmovies", "title", "id")
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Slug", "slug", db.Text, form.Text)
	formList.AddField("Region", "region", db.Text, form.Text)

	formList.SetTable("dbmovie_titles").SetTitle("DbmovieTitles").SetDescription("DbmovieTitles")

	return dbmovieTitles
}
