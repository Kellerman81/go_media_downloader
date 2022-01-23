package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func GetIndexerFailsTable(ctx *context.Context) table.Table {

	indexerFails := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	info := indexerFails.GetInfo().HideFilterArea()
	info.HideDeleteButton().HideEditButton().HideNewButton()

	info.AddField("Id", "id", db.Integer).
		FieldFilterable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Indexer", "indexer", db.Text)
	info.AddField("Last_fail", "last_fail", db.Datetime)

	info.SetTable("indexer_fails").SetTitle("IndexerFails").SetDescription("IndexerFails")

	formList := indexerFails.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default)
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Indexer", "indexer", db.Text, form.Text)
	formList.AddField("Last_fail", "last_fail", db.Datetime, form.Datetime)

	formList.SetTable("indexer_fails").SetTitle("IndexerFails").SetDescription("IndexerFails")

	return indexerFails
}
