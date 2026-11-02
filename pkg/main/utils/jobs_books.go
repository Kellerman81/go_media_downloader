package utils

import (
	"context"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/books"
)

func init() {
	books.RegisterRefresh(refreshBooksWrapper)
}

func refreshBooksWrapper(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if arr, ok := data.([]string); ok {
		return refreshbooks(ctx, cfgp, arr)
	}

	return nil
}

// refreshbooks refreshes book metadata by re-importing from configured providers for each ISBN-13.
func refreshbooks(ctx context.Context, cfgp *config.MediaTypeConfig, arr []string) error {
	if len(arr) == 0 {
		return nil
	}

	var err error
	for idx := range arr {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}

		logger.Logtype("info", 1).
			Str("isbn", arr[idx]).
			Msg("Refresh Book")

		listid := getrefreshbooklistid(&arr[idx], cfgp)
		if listid == -1 {
			continue
		}

		_, errsub := importfeed.JobImportBooks(
			ctx, arr[idx],
			cfgp,
			listid,
			false,
		)
		if errsub != nil {
			err = errsub
		}
	}

	return err
}

// RefreshBook refreshes the metadata of a single book by its dbbook ID.
func RefreshBook(cfgp *config.MediaTypeConfig, id *string) error {
	return refreshbooks(
		context.Background(),
		cfgp,
		database.GetrowsN[string](
			false,
			1,
			"select distinct dbbooks.isbn_13 from dbbooks inner join books on books.dbbook_id = dbbooks.id where dbbooks.id = ? and dbbooks.isbn_13 != ''",
			id,
		),
	)
}

// getrefreshbooklistid looks up the list ID for the given ISBN-13 and media config.
func getrefreshbooklistid(isbn *string, cfgp *config.MediaTypeConfig) int {
	listname := database.Getdatarow[string](
		false,
		"SELECT b.listname FROM books b JOIN dbbooks db ON b.dbbook_id = db.id WHERE db.isbn_13 = ?",
		isbn,
	)
	return cfgp.GetMediaListsEntryListID(listname)
}
