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

func getSeriesTable(ctx *context.Context) table.Table {
	series := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	info := series.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Title", "Seriename", db.Text).FieldJoin(types.Join{
		BaseTable: "series",
		Field:     "dbserie_id",
		JoinField: "id",
		Table:     "dbseries",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Listname", "listname", db.Text).
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	info.AddField("Rootpath", "rootpath", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike})

	info.AddColumnButtons(ctx, "Details", types.GetActionIconButton(
		icon.List,
		action.PopUpWithIframe(
			"/admin/info/serie_episodes",
			"see more",
			action.IframeData{
				Src: "/admin/info/serie_episodes",
				AddParameterFn: func(ctx *context.Context) string {
					return "&serie_id=" + ctx.FormValue("id")
				},
			},
			"900px",
			"560px",
		),
	), types.GetActionIconButton(
		icon.File,
		action.PopUpWithIframe(
			"/admin/info/serie_episode_files",
			"see more",
			action.IframeData{
				Src: "/admin/info/serie_episode_files",
				AddParameterFn: func(ctx *context.Context) string {
					return "&serie_id=" + ctx.FormValue("id")
				},
			},
			"900px",
			"560px",
		),
	), types.GetActionIconButton(
		icon.History,
		action.PopUpWithIframe(
			"/admin/info/serie_episode_histories",
			"see more",
			action.IframeData{
				Src: "/admin/info/serie_episode_histories",
				AddParameterFn: func(ctx *context.Context) string {
					return "&serie_id=" + ctx.FormValue("id")
				},
			},
			"900px",
			"560px",
		),
	), types.GetActionIconButton(
		icon.Search,
		myPopUpWithIframe(
			"/search",
			"see more",
			action.IframeData{
				Src: "/api/series/search/id/{{.Id}}?apikey=" + config.GetSettingsGeneral().WebAPIKey,
			},
			"900px",
			"560px",
		),
	))

	info.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
			return template.Default().
				Link().
				SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
				SetContent(template2.HTML(value.Value)).
				OpenInNewTab().
				SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
				GetContent()
		})
	info.AddField("Dont_upgrade", "dont_upgrade", db.Numeric).
		FieldBool("1", "0").
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptions(types.FieldOptions{
			{Value: "0", Text: "No"},
			{Value: "1", Text: "Yes"},
		}).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	info.AddField("Dont_search", "dont_search", db.Numeric).
		FieldBool("1", "0").
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptions(types.FieldOptions{
			{Value: "0", Text: "No"},
			{Value: "1", Text: "Yes"},
		}).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	info.AddField("Search_Specials", "search_specials", db.Numeric).
		FieldBool("1", "0").
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptions(types.FieldOptions{
			{Value: "0", Text: "No"},
			{Value: "1", Text: "Yes"},
		}).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	info.AddField("Ignore_runtime", "ignore_runtime", db.Numeric).
		FieldBool("1", "0").
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptions(types.FieldOptions{
			{Value: "0", Text: "No"},
			{Value: "1", Text: "Yes"},
		}).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()

	info.SetTable("series").SetTitle("Series").SetDescription("Series")

	formList := series.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Listname", "listname", db.Text, form.SelectSingle).
		FieldOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname").Select("listname") })
	// formList.AddField("Listname", "listname", db.Text, form.Text)
	formList.AddField("Rootpath", "rootpath", db.Text, form.Text)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("dbseries", "seriename", "id")
	formList.AddField("Dont_upgrade", "dont_upgrade", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Dont_search", "dont_search", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Search_specials", "search_specials", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Ignore_runtime", "ignore_runtime", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})

	formList.SetTable("series").SetTitle("Series").SetDescription("Series")

	return series
}

func getSerieEpisodesTable(ctx *context.Context) table.Table {
	serieEpisodes := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	detail := serieEpisodes.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Lastscan", "lastscan", db.Datetime)
	detail.AddField("Blacklisted", "blacklisted", db.Numeric)
	detail.AddField("Quality_reached", "quality_reached", db.Numeric)
	detail.AddField("Quality_profile", "quality_profile", db.Text)
	detail.AddField("Missing", "missing", db.Numeric)
	detail.AddField("Dont_upgrade", "dont_upgrade", db.Numeric)
	detail.AddField("Dont_search", "dont_search", db.Numeric)
	detail.AddField("Ignore_runtime", "ignore_runtime", db.Numeric)
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
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable() // .FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Identifier", "Identifier", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episodes",
		Field:     "dbserie_episode_id",
		JoinField: "id",
		Table:     "dbserie_episodes",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	// info.AddField("Lastscan", "lastscan", db.Datetime)
	// info.AddField("Blacklisted", "blacklisted", db.Numeric)
	info.AddField("Quality_reached", "quality_reached", db.Numeric).
		FieldBool("1", "0").
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptions(types.FieldOptions{
			{Value: "0", Text: "No"},
			{Value: "1", Text: "Yes"},
		}).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	info.AddField("Quality_profile", "quality_profile", db.Text).
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	info.AddField("Missing", "missing", db.Numeric).
		FieldBool("1", "0").
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptions(types.FieldOptions{
			{Value: "0", Text: "No"},
			{Value: "1", Text: "Yes"},
		}).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	info.AddField("Ignore_runtime", "ignore_runtime", db.Numeric).
		FieldBool("1", "0").
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptions(types.FieldOptions{
			{Value: "0", Text: "No"},
			{Value: "1", Text: "Yes"},
		}).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()

		// info.AddField("Dont_upgrade", "dont_upgrade", db.Numeric)
		// info.AddField("Dont_search", "dont_search", db.Numeric)
		// info.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
		// info.AddField("Serie_id", "serie_id", db.Integer)

	// info.AddColumnButtons("Details", types.GetColumnButton("Files", icon.File,
	// 	action.PopUpWithIframe("/admin/info/serie_episode_files", "see more", action.IframeData{Src: "/admin/info/serie_episode_files", AddParameterFn: func(ctx *context.Context) string {
	// 		return "&serie_episode_id=" + ctx.FormValue("id")
	// 	}}, "900px", "560px")), types.GetColumnButton("Histories", icon.Info,
	// 	action.PopUpWithIframe("/admin/info/serie_episode_histories", "see more", action.IframeData{Src: "/admin/info/serie_episode_histories", AddParameterFn: func(ctx *context.Context) string {
	// 		return "&serie_episode_id=" + ctx.FormValue("id")
	// 	}}, "900px", "560px")), types.GetColumnButton("Force Scan", icon.Search,
	// 	action.PopUpWithIframe("/search", "see more", action.IframeData{Src: "/api/series/episodes/search/id/{{.Id}}?apikey=" + cfg_general.WebApiKey}, "200px", "20px")))

	info.AddColumnButtons(ctx, "Details", types.GetActionIconButton(
		icon.File,
		action.PopUpWithIframe(
			"/admin/info/serie_episode_files",
			"see more",
			action.IframeData{
				Src: "/admin/info/serie_episode_files",
				AddParameterFn: func(ctx *context.Context) string {
					return "&serie_episode_id=" + ctx.FormValue("id")
				},
			},
			"900px",
			"560px",
		),
	), types.GetActionIconButton(
		icon.History,
		action.PopUpWithIframe(
			"/admin/info/serie_episode_histories",
			"see more",
			action.IframeData{
				Src: "/admin/info/serie_episode_histories",
				AddParameterFn: func(ctx *context.Context) string {
					return "&serie_episode_id=" + ctx.FormValue("id")
				},
			},
			"900px",
			"560px",
		),
	), types.GetActionIconButton(
		icon.Search, // action.JumpInNewTab("/api/series/episodes/search/id/{{.Id}}?apikey="+cfg_general.WebApiKey, "Search")))
		myPopUpWithIframe(
			"/search",
			"see more",
			action.IframeData{
				Src: "/api/series/episodes/search/id/{{.Id}}?apikey=" + config.GetSettingsGeneral().WebAPIKey,
			},
			"900px",
			"560px",
		),
	))

	info.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
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
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Lastscan", "lastscan", db.Datetime, form.Datetime)
	formList.AddField("Blacklisted", "blacklisted", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Quality_reached", "quality_reached", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).
		FieldOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })
	formList.AddField("Missing", "missing", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Dont_upgrade", "dont_upgrade", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Dont_search", "dont_search", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Ignore_runtime", "ignore_runtime", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField(
		"Dbserie_episode_id",
		"dbserie_episode_id",
		db.Integer,
		form.SelectSingle,
	) // .FieldOptionsFromTable("dbserie_episodes", "identifier", "id", func(sql *db.SQL) *db.SQL { return sql.Where("dbserie_id", "=", value.dbserie_id) })
	formList.AddField("Serie_id", "serie_id", db.Integer, form.Number)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("dbseries", "seriename", "id")

	formList.SetTable("serie_episodes").SetTitle("SerieEpisodes").SetDescription("SerieEpisodes")

	return serieEpisodes
}

func getSerieEpisodeFilesTable(ctx *context.Context) table.Table {
	serieEpisodeFiles := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	detail := serieEpisodeFiles.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	// detail.AddField("Created_at", "created_at", db.Datetime)
	// detail.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Location", "location", db.Text)
	detail.AddField("Filename", "filename", db.Text)
	detail.AddField("Extension", "extension", db.Text)
	detail.AddField("Quality_profile", "quality_profile", db.Text)
	detail.AddField("Proper", "proper", db.Numeric)
	detail.AddField("Extended", "extended", db.Numeric)
	detail.AddField("Repack", "repack", db.Numeric)
	detail.AddField("Height", "height", db.Integer)
	detail.AddField("Width", "width", db.Integer)
	detail.AddField("Resolution_id", "resolution_id", db.Integer)
	detail.AddField("Quality_id", "quality_id", db.Integer)
	detail.AddField("Codec_id", "codec_id", db.Integer)
	detail.AddField("Audio_id", "audio_id", db.Integer)
	detail.AddField("Serie_id", "serie_id", db.Integer)
	detail.AddField("Serie_episode_id", "serie_episode_id", db.Integer)
	detail.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
	detail.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
			return template.Default().
				Link().
				SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
				SetContent(template2.HTML(value.Value)).
				OpenInNewTab().
				SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
				GetContent()
		})

	detail.SetTable("serie_episode_files").
		SetTitle("SerieEpisodeFiles").
		SetDescription("SerieEpisodeFiles")

	info := serieEpisodeFiles.GetInfo().HideFilterArea()
	info.HideNewButton()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Location", "location", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	// info.AddField("Filename", "filename", db.Text)
	// info.AddField("Extension", "extension", db.Text)
	info.AddField("Quality_profile", "quality_profile", db.Text).
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	// info.AddField("Proper", "proper", db.Numeric)
	// info.AddField("Extended", "extended", db.Numeric)
	// info.AddField("Repack", "repack", db.Numeric)
	// info.AddField("Height", "height", db.Integer)
	// info.AddField("Width", "width", db.Integer)
	// info.AddField("Resolution_id", "resolution_id", db.Integer)
	// info.AddField("Quality_id", "quality_id", db.Integer)
	// info.AddField("Codec_id", "codec_id", db.Integer)
	// info.AddField("Audio_id", "audio_id", db.Integer)
	// info.AddField("Serie_id", "serie_id", db.Integer)
	info.AddField("List", "Listname", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episode_files",
		Field:     "serie_id",
		JoinField: "id",
		Table:     "series",
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	// info.AddField("Serie_episode_id", "serie_episode_id", db.Integer)
	// info.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
	info.AddField("Identifier", "Identifier", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episode_files",
		Field:     "dbserie_episode_id",
		JoinField: "id",
		Table:     "dbserie_episodes",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Title", "Seriename", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episode_files",
		Field:     "dbserie_id",
		JoinField: "id",
		Table:     "dbseries",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
			return template.Default().
				Link().
				SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
				SetContent(template2.HTML(value.Value)).
				OpenInNewTab().
				SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
				GetContent()
		})

	info.SetTable("serie_episode_files").
		SetTitle("SerieEpisodeFiles").
		SetDescription("SerieEpisodeFiles")

	formList := serieEpisodeFiles.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Location", "location", db.Text, form.Text)
	formList.AddField("Filename", "filename", db.Text, form.Text)
	formList.AddField("Extension", "extension", db.Text, form.Text)
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).
		FieldOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })
	formList.AddField("Proper", "proper", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Extended", "extended", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Repack", "repack", db.Numeric, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})
	formList.AddField("Height", "height", db.Integer, form.Number)
	formList.AddField("Width", "width", db.Integer, form.Number)
	formList.AddField("Resolution", "resolution_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "1") })
	formList.AddField("Quality", "quality_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "2") })
	formList.AddField("Codec", "codec_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "3") })
	formList.AddField("Audio", "audio_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "4") })
	formList.AddField("Serie_id", "serie_id", db.Integer, form.Number)
	formList.AddField("Serie_episode_id", "serie_episode_id", db.Integer, form.Number)
	formList.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer, form.Number)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("dbseries", "seriename", "id")

	formList.SetTable("serie_episode_files").
		SetTitle("SerieEpisodeFiles").
		SetDescription("SerieEpisodeFiles")

	return serieEpisodeFiles
}

func getSerieEpisodeHistoriesTable(ctx *context.Context) table.Table {
	serieEpisodeHistories := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	detail := serieEpisodeHistories.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	// detail.AddField("Created_at", "created_at", db.Datetime)
	// detail.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Title", "title", db.Text)
	detail.AddField("Url", "url", db.Text)
	detail.AddField("Indexer", "indexer", db.Text)
	detail.AddField("Type", "type", db.Text)
	detail.AddField("Target", "target", db.Text)
	detail.AddField("Downloaded_at", "downloaded_at", db.Datetime)
	detail.AddField("Blacklisted", "blacklisted", db.Numeric)
	detail.AddField("Quality_profile", "quality_profile", db.Text)
	detail.AddField("Resolution_id", "resolution_id", db.Integer)
	detail.AddField("Quality_id", "quality_id", db.Integer)
	detail.AddField("Codec_id", "codec_id", db.Integer)
	detail.AddField("Audio_id", "audio_id", db.Integer)
	detail.AddField("Serie_id", "serie_id", db.Integer)
	detail.AddField("Serie_episode_id", "serie_episode_id", db.Integer)
	detail.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
	detail.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
			return template.Default().
				Link().
				SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
				SetContent(template2.HTML(value.Value)).
				OpenInNewTab().
				SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
				GetContent()
		})

	detail.SetTable("serie_episode_histories").
		SetTitle("SerieEpisodeHistories").
		SetDescription("SerieEpisodeHistories")

	info := serieEpisodeHistories.GetInfo().HideFilterArea()
	info.HideNewButton()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Title", "title", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	// info.AddField("Url", "url", db.Text)
	info.AddField("Indexer", "indexer", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	// info.AddField("Type", "type", db.Text)
	// info.AddField("Target", "target", db.Text)
	info.AddField("Downloaded_at", "downloaded_at", db.Datetime).
		FieldSortable().
		FieldFilterable(types.FilterType{FormType: form.DatetimeRange})
	// info.AddField("Blacklisted", "blacklisted", db.Numeric)
	info.AddField("Quality_profile", "quality_profile", db.Text).
		FieldFilterable(types.FilterType{FormType: form.SelectSingle}).
		FieldFilterOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).
		FieldFilterOptionExt(map[string]any{"allowClear": true}).
		FieldSortable()
	// info.AddField("Resolution_id", "resolution_id", db.Integer)
	// info.AddField("Quality_id", "quality_id", db.Integer)
	// info.AddField("Codec_id", "codec_id", db.Integer)
	// info.AddField("Audio_id", "audio_id", db.Integer)
	// info.AddField("Serie_id", "serie_id", db.Integer)
	// info.AddField("Serie_episode_id", "serie_episode_id", db.Integer)
	info.AddField("List", "Listname", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episode_histories",
		Field:     "serie_id",
		JoinField: "id",
		Table:     "series",
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	info.AddField("Identifier", "Identifier", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episode_histories",
		Field:     "dbserie_episode_id",
		JoinField: "id",
		Table:     "dbserie_episodes",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Title", "Seriename", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episode_histories",
		Field:     "dbserie_id",
		JoinField: "id",
		Table:     "dbseries",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
	info.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
			return template.Default().
				Link().
				SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
				SetContent(template2.HTML(value.Value)).
				OpenInNewTab().
				SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
				GetContent()
		})

	info.SetTable("serie_episode_histories").
		SetTitle("SerieEpisodeHistories").
		SetDescription("SerieEpisodeHistories")

	formList := serieEpisodeHistories.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Url", "url", db.Text, form.Text)
	formList.AddField("Indexer", "indexer", db.Text, form.Text)
	formList.AddField("Type", "type", db.Text, form.Text)
	formList.AddField("Target", "target", db.Text, form.Text)
	formList.AddField("Downloaded_at", "downloaded_at", db.Datetime, form.Datetime)
	formList.AddField("Blacklisted", "blacklisted", db.Numeric, form.Switch).
		FieldOptions(types.FieldOptions{
			{Text: "Yes", Value: "1"},
			{Text: "No", Value: "0"},
		})
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).
		FieldOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })
	formList.AddField("Resolution", "resolution_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "1") })
	formList.AddField("Quality", "quality_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "2") })
	formList.AddField("Codec", "codec_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "3") })
	formList.AddField("Audio", "audio_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "4") })
	formList.AddField("Serie_id", "serie_id", db.Integer, form.Number)
	formList.AddField("Serie_episode_id", "serie_episode_id", db.Integer, form.Number)
	formList.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer, form.Number)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("dbseries", "seriename", "id")

	formList.SetTable("serie_episode_histories").
		SetTitle("SerieEpisodeHistories").
		SetDescription("SerieEpisodeHistories")

	return serieEpisodeHistories
}

func getSerieFileUnmatchedsTable(ctx *context.Context) table.Table {
	serieFileUnmatcheds := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	info := serieFileUnmatcheds.GetInfo().HideFilterArea()
	info.HideDeleteButton().HideEditButton().HideNewButton()

	info.AddField("Id", "id", db.Integer).FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime).FieldDate("2006-01-02 15:04")
	info.AddField("Listname", "listname", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Filepath", "filepath", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Last_checked", "last_checked", db.Datetime).FieldFilterable().FieldSortable()
	info.AddField("Parsed_data", "parsed_data", db.Text)

	info.SetTable("serie_file_unmatcheds").
		SetTitle("SerieFileUnmatcheds").
		SetDescription("SerieFileUnmatcheds")

	formList := serieFileUnmatcheds.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Listname", "listname", db.Text, form.Text)
	formList.AddField("Filepath", "filepath", db.Text, form.Text)
	formList.AddField("Last_checked", "last_checked", db.Datetime, form.Datetime)
	formList.AddField("Parsed_data", "parsed_data", db.Text, form.Text)

	formList.SetTable("serie_file_unmatcheds").
		SetTitle("SerieFileUnmatcheds").
		SetDescription("SerieFileUnmatcheds")

	return serieFileUnmatcheds
}

func getDbseriesTable(ctx *context.Context) table.Table {
	dbseries := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	detail := dbseries.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	// detail.AddField("Created_at", "created_at", db.Datetime)
	// detail.AddField("Updated_at", "updated_at", db.Datetime)
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
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Seriename", "seriename", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	// info.AddField("Aliases", "aliases", db.Text)
	// info.AddField("Season", "season", db.Text)
	info.AddField("Status", "status", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Firstaired", "firstaired", db.Text)
	// info.AddField("Network", "network", db.Text)
	// info.AddField("Runtime", "runtime", db.Text)
	// info.AddField("Language", "language", db.Text)
	info.AddField("Genre", "genre", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike})
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

	info.AddColumnButtons(ctx, "Details", types.GetColumnButton(
		"Titles",
		icon.Info,
		action.PopUpWithIframe(
			"/admin/info/dbserie_alternates",
			"see more",
			action.IframeData{
				Src: "/admin/info/dbserie_alternates",
				AddParameterFn: func(ctx *context.Context) string {
					return "&dbserie_id=" + ctx.FormValue("id")
				},
			},
			"900px",
			"560px",
		),
	), types.GetColumnButton(
		"Episodes",
		icon.Info,
		action.PopUpWithIframe(
			"/admin/info/dbserie_episodes",
			"see more",
			action.IframeData{
				Src: "/admin/info/dbserie_episodes",
				AddParameterFn: func(ctx *context.Context) string {
					return "&dbserie_id=" + ctx.FormValue("id")
				},
			},
			"900px",
			"560px",
		),
	), types.GetColumnButton(
		"Shows",
		icon.Info,
		action.PopUpWithIframe(
			"/admin/info/series",
			"see more",
			action.IframeData{
				Src: "/admin/info/series",
				AddParameterFn: func(ctx *context.Context) string {
					return "&dbserie_id=" + ctx.FormValue("id")
				},
			},
			"900px",
			"560px",
		),
	), types.GetColumnButton(
		"Refresh",
		icon.Refresh,
		myPopUpWithIframe(
			"/admin/info/refresh",
			"see more",
			action.IframeData{
				Src: "/api/series/refresh/{{.Id}}?apikey=" + config.GetSettingsGeneral().WebAPIKey,
			},
			"900px",
			"560px",
		),
	))
	info.SetTable("dbseries").SetTitle("Dbseries").SetDescription("Dbseries")

	formList := dbseries.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
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

func getDbserieEpisodesTable(ctx *context.Context) table.Table {
	dbserieEpisodes := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	detail := dbserieEpisodes.GetDetail()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable().FieldSortable()
	detail.AddField("Created_at", "created_at", db.Datetime)
	detail.AddField("Updated_at", "updated_at", db.Datetime)
	detail.AddField("Episode", "episode", db.Text).FieldSortable()
	detail.AddField("Season", "season", db.Text).FieldSortable()
	detail.AddField("Identifier", "identifier", db.Text)
	detail.AddField("Title", "title", db.Text)
	detail.AddField("Overview", "overview", db.Text)
	detail.AddField("Poster", "poster", db.Text)
	detail.AddField("Seriename", "seriename", db.Text).FieldJoin(types.Join{
		BaseTable: "dbserie_episodes",
		Field:     "dbserie_id",
		JoinField: "id",
		Table:     "dbseries",
	}).FieldSortable()
	detail.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
			return template.Default().
				Link().
				SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
				SetContent(template2.HTML(value.Value)).
				OpenInNewTab().
				SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
				GetContent()
		})
	detail.AddField("First_aired", "first_aired", db.Datetime)
	detail.AddField("Runtime", "runtime", db.Integer)

	detail.SetTable("dbserie_episodes").
		SetTitle("DbserieEpisodes").
		SetDescription("DbserieEpisodes")

	info := dbserieEpisodes.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	info.AddField("Seriename", "seriename", db.Text).FieldJoin(types.Join{
		BaseTable: "dbserie_episodes",
		Field:     "dbserie_id",
		JoinField: "id",
		Table:     "dbseries",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	// info.AddField("Episode", "episode", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Season", "season", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Identifier", "identifier", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Title", "title", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	// info.AddField("Overview", "overview", db.Text)
	// info.AddField("Poster", "poster", db.Text)
	info.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
			return template.Default().
				Link().
				SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
				SetContent(template2.HTML(value.Value)).
				OpenInNewTab().
				SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
				GetContent()
		})

		// info.AddField("First_aired", "first_aired", db.Datetime)
		// info.AddField("Runtime", "runtime", db.Integer)

	info.SetTable("dbserie_episodes").SetTitle("DbserieEpisodes").SetDescription("DbserieEpisodes")

	formList := dbserieEpisodes.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Episode", "episode", db.Text, form.Text)
	formList.AddField("Season", "season", db.Text, form.Text)
	formList.AddField("Identifier", "identifier", db.Text, form.Text)
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Overview", "overview", db.Text, form.RichText)
	formList.AddField("Poster", "poster", db.Text, form.Text)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("dbseries", "seriename", "id")
	formList.AddField("First_aired", "first_aired", db.Datetime, form.Datetime)
	formList.AddField("Runtime", "runtime", db.Integer, form.Number)

	formList.SetTable("dbserie_episodes").
		SetTitle("DbserieEpisodes").
		SetDescription("DbserieEpisodes")

	return dbserieEpisodes
}

func getDbserieAlternatesTable(ctx *context.Context) table.Table {
	dbserieAlternates := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	info := dbserieAlternates.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Title", "title", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Slug", "slug", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Dbserie_id", "dbserie_id", db.Integer).
		FieldDisplay(func(value types.FieldModel) any {
			return template.Default().
				Link().
				SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
				SetContent(template2.HTML(value.Value)).
				OpenInNewTab().
				SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
				GetContent()
		})
	info.AddField("Region", "region", db.Text)

	info.SetTable("dbserie_alternates").
		SetTitle("DbserieAlternates").
		SetDescription("DbserieAlternates")

	formList := dbserieAlternates.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Slug", "slug", db.Text, form.Text)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).
		FieldOptionsFromTable("dbseries", "seriename", "id")
	formList.AddField("Region", "region", db.Text, form.Text)

	formList.SetTable("dbserie_alternates").
		SetTitle("DbserieAlternates").
		SetDescription("DbserieAlternates")

	return dbserieAlternates
}
