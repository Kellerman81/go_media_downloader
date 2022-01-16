CREATE INDEX "idx_imdb_akas_tconst" ON "imdb_akas" (
	"tconst"
);
CREATE INDEX "idx_imdb_titles_start_year" ON "imdb_titles" (
	"start_year"
);
CREATE INDEX "idx_imdb_titles_original_title" ON "imdb_titles" (
	"original_title"
);
CREATE INDEX "idx_imdb_akas_title_slug" ON "imdb_akas" (
	"title",
	"slug"
);
CREATE INDEX "idx_imdb_titles_primary_original_slug" ON "imdb_titles" (
	"primary_title",
	"original_title",
	"slug"
);