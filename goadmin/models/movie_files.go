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

func GetMovieFilesTable(ctx *context.Context) table.Table {

	movieFiles := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

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

	detail.AddField("Dbmovie_id", "dbmovie_id", db.Int).FieldDisplay(func(value types.FieldModel) interface{} {
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
		FieldFilterable().FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Location", "location", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	// info.AddField("Filename", "filename", db.Text)
	// info.AddField("Extension", "extension", db.Text)
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
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Title", "title", db.Text).FieldJoin(types.Join{
		BaseTable: "movie_files",
		Field:     "dbmovie_id",
		Table:     "dbmovies",
		JoinField: "id",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()

	info.AddField("Dbmovie_id", "dbmovie_id", db.Int).FieldDisplay(func(value types.FieldModel) interface{} {
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
