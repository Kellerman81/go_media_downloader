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

func GetDbmovieTitlesTable(ctx *context.Context) table.Table {

	dbmovieTitles := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	info := dbmovieTitles.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Dbmovie_id", "dbmovie_id", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
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
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Dbmovie_id", "dbmovie_id", db.Integer, form.SelectSingle).FieldOptionsFromTable("dbmovies", "title", "id")
	formList.AddField("Title", "title", db.Text, form.Text)
	formList.AddField("Slug", "slug", db.Text, form.Text)
	formList.AddField("Region", "region", db.Text, form.Text)

	formList.SetTable("dbmovie_titles").SetTitle("DbmovieTitles").SetDescription("DbmovieTitles")

	return dbmovieTitles
}
