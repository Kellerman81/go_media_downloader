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

func GetDbserieEpisodesTable(ctx *context.Context) table.Table {

	dbserieEpisodes := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

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
	detail.AddField("Dbserie_id", "dbserie_id", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
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

	detail.SetTable("dbserie_episodes").SetTitle("DbserieEpisodes").SetDescription("DbserieEpisodes")

	info := dbserieEpisodes.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).
		FieldFilterable().FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Episode", "episode", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Season", "season", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	//info.AddField("Identifier", "identifier", db.Text)
	info.AddField("Title", "title", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	//info.AddField("Overview", "overview", db.Text)
	//info.AddField("Poster", "poster", db.Text)
	info.AddField("Seriename", "seriename", db.Text).FieldJoin(types.Join{
		BaseTable: "dbserie_episodes",
		Field:     "dbserie_id",
		JoinField: "id",
		Table:     "dbseries",
	}).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Dbserie_id", "dbserie_id", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
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
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Episode", "episode", db.Text, form.Text)
	formList.AddField("Season", "season", db.Text, form.Text)
	formList.AddField("Identifier", "identifier", db.Text, form.Text)
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Overview", "overview", db.Text, form.RichText)
	formList.AddField("Poster", "poster", db.Text, form.Text)
	formList.AddField("Dbserie_id", "dbserie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbseries", "seriename", "id")
	formList.AddField("First_aired", "first_aired", db.Datetime, form.Datetime)
	formList.AddField("Runtime", "runtime", db.Integer, form.Number)

	formList.SetTable("dbserie_episodes").SetTitle("DbserieEpisodes").SetDescription("DbserieEpisodes")

	return dbserieEpisodes
}
