package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func getJobHistoriesTable(ctx *context.Context) table.Table {
	jobHistories := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriverAndConnection("sqlite", "media"),
	)

	info := jobHistories.GetInfo().HideFilterArea()
	info.SetAutoRefresh(60)
	info.HideDeleteButton().HideEditButton().HideNewButton()

	info.AddField("Id", "id", db.Integer).
		FieldSortable()
	// info.AddField("Created_at", "created_at", db.Datetime)
	// info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Job_type", "job_type", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Job_category", "job_category", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Job_group", "job_group", db.Text).
		FieldFilterable(types.FilterType{Operator: types.FilterOperatorLike}).
		FieldSortable()
	info.AddField("Started", "started", db.Datetime).
		FieldSortable().
		FieldFilterable(types.FilterType{FormType: form.DatetimeRange})
	info.AddField("Ended", "ended", db.Datetime).
		FieldSortable().
		FieldFilterable(types.FilterType{FormType: form.DatetimeRange})

	info.SetTable("job_histories").SetTitle("JobHistories").SetDescription("JobHistories")

	formList := jobHistories.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).
		FieldDisplayButCanNotEditWhenCreate().
		FieldDisableWhenUpdate()
	// formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	// formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Job_type", "job_type", db.Text, form.Text)
	formList.AddField("Job_category", "job_category", db.Text, form.Text)
	formList.AddField("Job_group", "job_group", db.Text, form.Text)
	formList.AddField("Started", "started", db.Datetime, form.Datetime)
	formList.AddField("Ended", "ended", db.Datetime, form.Datetime)

	formList.SetTable("job_histories").SetTitle("JobHistories").SetDescription("JobHistories")

	return jobHistories
}
