package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func getRSshistoriesTable(ctx *context.Context) table.Table {
	rSshistories := table.NewDefaultTable(ctx, table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	info := rSshistories.GetInfo()

	info.AddField("Id", "id", db.Integer)
	info.AddField("Created_at", "created_at", db.Datetime)
	info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Config", "config", db.Text)
	info.AddField("List", "list", db.Text)
	info.AddField("Indexer", "indexer", db.Text)
	info.AddField("Last_id", "last_id", db.Text)

	info.SetTable("r_sshistories").SetTitle("RSshistories").SetDescription("RSshistories")

	formList := rSshistories.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisableWhenCreate()
	formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime).
		FieldHide().FieldNowWhenInsert()
	formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime).
		FieldHide().FieldNowWhenUpdate()
	formList.AddField("Config", "config", db.Text, form.RichText)
	formList.AddField("List", "list", db.Text, form.RichText)
	formList.AddField("Indexer", "indexer", db.Text, form.RichText)
	formList.AddField("Last_id", "last_id", db.Text, form.RichText)

	formList.SetTable("r_sshistories").SetTitle("RSshistories").SetDescription("RSshistories")

	return rSshistories
}
