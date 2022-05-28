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

func GetSerieEpisodeHistoriesTable(ctx *context.Context) table.Table {

	serieEpisodeHistories := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	detail := serieEpisodeHistories.GetDetail().HideFilterArea()

	detail.AddField("Id", "id", db.Integer).
		FieldFilterable()
	//detail.AddField("Created_at", "created_at", db.Datetime)
	//detail.AddField("Updated_at", "updated_at", db.Datetime)
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
	detail.AddField("Dbserie_id", "dbserie_id", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
		return template.Default().
			Link().
			SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
			GetContent()
	})

	detail.SetTable("serie_episode_histories").SetTitle("SerieEpisodeHistories").SetDescription("SerieEpisodeHistories")

	info := serieEpisodeHistories.GetInfo().HideFilterArea()
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
	info.AddField("Quality_profile", "quality_profile", db.Text).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile") }).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
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
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptionsFromTable("series", "listname", "listname", func(sql *db.SQL) *db.SQL { return sql.GroupBy("listname") }).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
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
	info.AddField("Dbserie_id", "dbserie_id", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
		return template.Default().
			Link().
			SetURL("/admin/info/dbseries/detail?__goadmin_detail_pk=" + value.Value).
			SetContent(template2.HTML(value.Value)).
			OpenInNewTab().
			SetTabTitle(template.HTML("Serie Detail(" + value.Value + ")")).
			GetContent()
	})

	info.SetTable("serie_episode_histories").SetTitle("SerieEpisodeHistories").SetDescription("SerieEpisodeHistories")

	formList := serieEpisodeHistories.GetForm()
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
	formList.AddField("Quality_profile", "quality_profile", db.Text, form.SelectSingle).FieldOptionsFromTable("serie_episodes", "quality_profile", "quality_profile", func(sql *db.SQL) *db.SQL { return sql.GroupBy("quality_profile").Select("quality_profile") })
	formList.AddField("Resolution", "resolution_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "1") })
	formList.AddField("Quality", "quality_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "2") })
	formList.AddField("Codec", "codec_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "3") })
	formList.AddField("Audio", "audio_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("qualities", "name", "id", func(sql *db.SQL) *db.SQL { return sql.Where("type", "=", "4") })
	formList.AddField("Serie_id", "serie_id", db.Integer, form.Number)
	formList.AddField("Serie_episode_id", "serie_episode_id", db.Integer, form.Number)
	formList.AddField("Dbserie_episode_id", "dbserie_episode_id", db.Integer, form.Number)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbseries", "seriename", "id")

	formList.SetTable("serie_episode_histories").SetTitle("SerieEpisodeHistories").SetDescription("SerieEpisodeHistories")

	return serieEpisodeHistories
}
