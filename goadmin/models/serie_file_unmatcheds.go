package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func GetSerieFileUnmatchedsTable(ctx *context.Context) table.Table {

	serieFileUnmatcheds := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	info := serieFileUnmatcheds.GetInfo().HideFilterArea()
	info.HideDeleteButton().HideEditButton().HideNewButton()

	info.AddField("Id", "id", db.Integer).FieldFilterable().FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Listname", "listname", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Filepath", "filepath", db.Text).FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).FieldSortable()
	info.AddField("Last_checked", "last_checked", db.Datetime).FieldFilterable().FieldSortable()
	info.AddField("Parsed_data", "parsed_data", db.Text)

	info.SetTable("serie_file_unmatcheds").SetTitle("SerieFileUnmatcheds").SetDescription("SerieFileUnmatcheds")

	formList := serieFileUnmatcheds.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Listname", "listname", db.Text, form.Text)
	formList.AddField("Filepath", "filepath", db.Text, form.Text)
	formList.AddField("Last_checked", "last_checked", db.Datetime, form.Datetime)
	formList.AddField("Parsed_data", "parsed_data", db.Text, form.Text)

	formList.SetTable("serie_file_unmatcheds").SetTitle("SerieFileUnmatcheds").SetDescription("SerieFileUnmatcheds")

	return serieFileUnmatcheds
}
