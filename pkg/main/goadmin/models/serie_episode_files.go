package models

import (
	template2 "html/template"

	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func GetSerieEpisodeFilesTable(ctx *context.Context) table.Table {
	serieEpisodeFiles := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := serieEpisodeFiles.GetDetail().HideFilterArea()

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
	detail.AddField("Resolution_id", "resolution_id", db.Integer)
	detail.AddField("Quality_id", "quality_id", db.Integer)
	detail.AddField("Codec_id", "codec_id", db.Integer)
	detail.AddField("Audio_id", "audio_id", db.Integer)
	detail.AddField("Serie_id", "serie_id", db.Integer)
	detail.AddField("Serie_episode_id", "serie_episode_id", db.Integer)
	detail.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
	detail.AddField("Dbserie_id", "dbserie_id", db.Integer).FieldDisplay(func(value types.FieldModel) any {
		return template.Default().
			Link().
			SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
			GetContent()
	})

	detail.SetTable("serie_episode_files").SetTitle("SerieEpisodeFiles").SetDescription("SerieEpisodeFiles")

	info := serieEpisodeFiles.GetInfo().HideFilterArea()
	info.HideNewButton()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Location", "location", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Filename", "filename", db.Text)
	// info.AddField("Extension", "extension", db.Text)
	info.AddField("Quality_profile", "quality_profile", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	// info.AddField("Proper", "proper", db.Numeric)
	// info.AddField("Extended", "extended", db.Numeric)
	// info.AddField("Repack", "repack", db.Numeric)
	// info.AddField("Height", "height", db.Integer)
	// info.AddField("Width", "width", db.Integer)
	// info.AddField("Resolution_id", "resolution_id", db.Integer)
	// info.AddField("Quality_id", "quality_id", db.Integer)
	// info.AddField("Codec_id", "codec_id", db.Integer)
	// info.AddField("Audio_id", "audio_id", db.Integer)
	//info.AddField("Serie_id", "serie_id", db.Integer)
	info.AddField("List", "Listname", db.Text).FieldJoin(types.Join{
		BaseTable: "serie_episode_files",
		Field:     "serie_id",
		JoinField: "id",
		Table:     "series",
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]any{"allowClear": true}).FieldSortable()
	// info.AddField("Serie_episode_id", "serie_episode_id", db.Integer)
	//info.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer)
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
	info.AddField("Dbserie_id", "dbserie_id", db.Integer).FieldDisplay(func(value types.FieldModel) any {
		return template.Default().
			Link().
			SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
			GetContent()
	})

	info.SetTable("serie_episode_files").SetTitle("SerieEpisodeFiles").SetDescription("SerieEpisodeFiles")

	formList := serieEpisodeFiles.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Location", "location", db.Text, form.Text)
	formList.AddField("Filename", "filename", db.Text, form.Text)
	formList.AddField("Extension", "extension", db.Text, form.Text)
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).FieldOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })
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
	})
	formList.AddField("Height", "height", db.Integer, form.Number)
	formList.AddField("Width", "width", db.Integer, form.Number)
	formList.AddField("Resolution", "resolution_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "1") })
	formList.AddField("Quality", "quality_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "2") })
	formList.AddField("Codec", "codec_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "3") })
	formList.AddField("Audio", "audio_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "4") })
	formList.AddField("Serie_id", "serie_id", db.Integer, form.Number)
	formList.AddField("Serie_episode_id", "serie_episode_id", db.Integer, form.Number)
	formList.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer, form.Number)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbseries", "seriename", "id")

	formList.SetTable("serie_episode_files").SetTitle("SerieEpisodeFiles").SetDescription("SerieEpisodeFiles")

	return serieEpisodeFiles
}
