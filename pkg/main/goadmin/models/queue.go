package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/parameter"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

func getQueueTable(ctx *context.Context) (userTable table.Table) {
	userTable = table.NewDefaultTable(ctx, table.Config{})
	userTable.GetOnlyInfo()
	var queue []map[string]any
	i := 0
	for _, value := range worker.GetQueues() {
		queue = append(queue, map[string]any{
			"id":      i,
			"queue":   value.Queue,
			"job":     value.Name,
			"added":   value.Added.Format("2006-01-02 15:04:05"),
			"started": value.Started.Format("2006-01-02 15:04:05"),
		})
		i++
	}

	info := userTable.GetInfo().SetDefaultPageSize(100)
	info.SetAutoRefresh(10)
	info.HideDeleteButton().HideDetailButton().HideEditButton().HideExportButton().HideFilterArea().HideFilterButton().HideNewButton().HidePagination()
	info.AddField("ID", "id", db.Numeric)
	info.AddField("Queue", "queue", db.Varchar)
	info.AddField("Job", "job", db.Varchar)
	info.AddField("Added", "added", db.Datetime)
	info.AddField("Started", "started", db.Datetime)

	info.SetTable("Queue").
		SetTitle("Queue").
		SetDescription("Queue").
		SetGetDataFn(func(param parameter.Parameters) (data []map[string]any, size int) {
			param.PK()

			return queue, len(queue)
		})
	return userTable
}
