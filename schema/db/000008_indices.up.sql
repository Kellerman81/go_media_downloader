CREATE INDEX "idx_dbmovies_imdb_id" ON "dbmovies" (
	"imdb_id"
);
CREATE INDEX "idx_dbmovies_title" ON "dbmovies" (
	"title"
);
CREATE INDEX "idx_dbmovies_slug" ON "dbmovies" (
	"slug"
);
CREATE INDEX "idx_dbmovies_title_slug" ON "dbmovies" (
	"title",
    "slug"
);
CREATE INDEX "idx_dbmovie_titles_title" ON "dbmovie_titles" (
	"title"
);
CREATE INDEX "idx_dbmovie_titles_slug" ON "dbmovie_titles" (
	"slug"
);
CREATE INDEX "idx_dbmovie_titles_title_slug" ON "dbmovie_titles" (
	"title",
    "slug"
);
CREATE INDEX "idx_dbserie_alternates_slug" ON "dbserie_alternates" (
	"slug"
);

CREATE INDEX "idx_dbseries_seriename" ON "dbseries" (
	"seriename"
);
CREATE INDEX "idx_dbseries_slug" ON "dbseries" (
	"slug"
);
CREATE INDEX "idx_dbseries_tvdbid" ON "dbseries" (
	"thetvdb_id"
);

CREATE INDEX "idx_series_listname" ON "series" (
    "listname"
);
CREATE INDEX "idx_series_dbserie_id_listname" ON "series" (
	"dbserie_id",
    "listname"
);

CREATE INDEX "idx_movies_listname" ON "movies" (
    "listname"
);
CREATE INDEX "idx_movies_dbmovie_id_listname" ON "movies" (
	"dbmovie_id",
    "listname"
);